package services

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// DockerService handles Docker operations
type DockerService struct {
	client *client.Client
}

// NewDockerService creates a new Docker service
func NewDockerService() (*DockerService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &DockerService{client: cli}, nil
}

// BuildImage builds a Docker image from a Dockerfile
func (s *DockerService) BuildImage(ctx context.Context, buildContext io.Reader, tag string) (string, error) {
	opts := types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: "Dockerfile",
		Remove:     true,
		ForceRemove: true,
	}

	resp, err := s.client.ImageBuild(ctx, buildContext, opts)
	if err != nil {
		return "", fmt.Errorf("failed to build image: %w", err)
	}
	defer resp.Body.Close()

	// Read build output
	output, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read build output: %w", err)
	}

	return string(output), nil
}

// CreateContainer creates a Docker container
func (s *DockerService) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkConfig *network.NetworkingConfig, containerName string) (string, error) {
	resp, err := s.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

// StartContainer starts a Docker container
func (s *DockerService) StartContainer(ctx context.Context, containerID string) error {
	if err := s.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	return nil
}

// StopContainer stops a Docker container
func (s *DockerService) StopContainer(ctx context.Context, containerID string) error {
	timeout := 30
	if err := s.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	return nil
}

// RemoveContainer removes a Docker container
func (s *DockerService) RemoveContainer(ctx context.Context, containerID string) error {
	if err := s.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}
	return nil
}

// GetContainersByLabel finds containers with a specific label
func (s *DockerService) GetContainersByLabel(ctx context.Context, labelKey, labelValue string) ([]types.Container, error) {
	filter := fmt.Sprintf("%s=%s", labelKey, labelValue)
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", filter)
	containers, err := s.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}

// GetContainerLogs retrieves logs from a container
func (s *DockerService) GetContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	}

	reader, err := s.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer reader.Close()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	return string(logs), nil
}

// NetworkExists checks if a Docker network exists
func (s *DockerService) NetworkExists(ctx context.Context, networkName string) (bool, error) {
	networks, err := s.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list networks: %w", err)
	}

	for _, network := range networks {
		if network.Name == networkName {
			return true, nil
		}
	}

	return false, nil
}

// CreateNetwork creates a Docker network
func (s *DockerService) CreateNetwork(ctx context.Context, networkName string) error {
	_, err := s.client.NetworkCreate(ctx, networkName, types.NetworkCreate{})
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}
	return nil
}

// GenerateTraefikLabels generates Traefik labels for a container
func GenerateTraefikLabels(appName, domain string, port int, enableTLS bool) map[string]string {
	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", appName): fmt.Sprintf("Host(`%s`)", domain),
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", appName): fmt.Sprintf("%d", port),
	}

	if enableTLS && domain != "" {
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", appName)] = "websecure"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", appName)] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", appName)] = "letsencrypt"
	} else {
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", appName)] = "web"
	}

	return labels
}

// CreateContainerConfig creates a container configuration with Traefik labels
func CreateContainerConfig(imageName string, envVars map[string]string, appName string, domain string, port int, networks []string) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	// Convert env vars to Docker format
	env := []string{}
	for key, value := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Generate Traefik labels
	labels := GenerateTraefikLabels(appName, domain, port, domain != "")
	labels["atmosphere.app"] = appName

	// Container config
	config := &container.Config{
		Image:  imageName,
		Env:    env,
		Labels: labels,
		ExposedPorts: nat.PortSet{
			nat.Port(fmt.Sprintf("%d/tcp", port)): struct{}{},
		},
	}

	// Host config
	hostConfig := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	// Network config
	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}

	for _, net := range networks {
		networkConfig.EndpointsConfig[net] = &network.EndpointSettings{}
	}

	return config, hostConfig, networkConfig
}
