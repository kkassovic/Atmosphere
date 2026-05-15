package services

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
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

// GetContainersByComposeProject finds containers created under a specific compose project.
func (s *DockerService) GetContainersByComposeProject(ctx context.Context, projectName string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", fmt.Sprintf("com.docker.compose.project=%s", projectName))

	containers, err := s.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list compose project containers: %w", err)
	}

	return containers, nil
}

// GetContainersByNamePrefix finds containers with names matching a prefix.
func (s *DockerService) GetContainersByNamePrefix(ctx context.Context, prefix string) ([]types.Container, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("name", prefix)

	containers, err := s.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers by name prefix: %w", err)
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

// EnsureVolume ensures a Docker volume exists.
func (s *DockerService) EnsureVolume(ctx context.Context, volumeName string) error {
	_, err := s.client.VolumeInspect(ctx, volumeName)
	if err == nil {
		return nil
	}

	_, err = s.client.VolumeCreate(ctx, volume.CreateOptions{Name: volumeName})
	if err != nil {
		return fmt.Errorf("failed to create volume %s: %w", volumeName, err)
	}

	return nil
}

// GetVolumeMountpoint returns the host mountpoint path for a named Docker volume.
func (s *DockerService) GetVolumeMountpoint(ctx context.Context, volumeName string) (string, error) {
	if volumeName == "" {
		return "", fmt.Errorf("volume name is required")
	}

	v, err := s.client.VolumeInspect(ctx, volumeName)
	if err != nil {
		return "", fmt.Errorf("failed to inspect volume %s: %w", volumeName, err)
	}
	if v.Mountpoint == "" {
		return "", fmt.Errorf("volume %s has empty mountpoint", volumeName)
	}

	return v.Mountpoint, nil
}

// RemoveVolume removes a Docker volume and its data.
func (s *DockerService) RemoveVolume(ctx context.Context, volumeName string) error {
	if volumeName == "" {
		return fmt.Errorf("volume name is required")
	}

	if err := s.client.VolumeRemove(ctx, volumeName, true); err != nil {
		return fmt.Errorf("failed to remove volume %s: %w", volumeName, err)
	}

	return nil
}

// GetVolumeNamesByApp returns unique named Docker volume names mounted by app containers.
func (s *DockerService) GetVolumeNamesByApp(ctx context.Context, appName string) ([]string, error) {
	containersByAppLabel, err := s.GetContainersByLabel(ctx, "atmosphere.app", appName)
	if err != nil {
		return nil, err
	}

	projectName := fmt.Sprintf("atmosphere-%s", appName)
	containersByComposeProject, err := s.GetContainersByComposeProject(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to list compose project containers: %w", err)
	}

	containers := append(containersByAppLabel, containersByComposeProject...)

	volumeMap := make(map[string]struct{})
	for _, c := range containers {
		inspect, err := s.client.ContainerInspect(ctx, c.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to inspect container %s: %w", c.ID[:12], err)
		}

		for _, m := range inspect.Mounts {
			if m.Type == mount.TypeVolume && m.Name != "" {
				volumeMap[m.Name] = struct{}{}
			}
		}
	}

	volumes := make([]string, 0, len(volumeMap))
	for name := range volumeMap {
		volumes = append(volumes, name)
	}
	sort.Strings(volumes)

	return volumes, nil
}

// GetAppRuntimeIssues inspects app containers and returns runtime issues.
func (s *DockerService) GetAppRuntimeIssues(ctx context.Context, appName string) ([]string, error) {
	containersByAppLabel, err := s.GetContainersByLabel(ctx, "atmosphere.app", appName)
	if err != nil {
		return nil, err
	}

	projectName := fmt.Sprintf("atmosphere-%s", appName)
	containersByComposeProject, err := s.GetContainersByComposeProject(ctx, projectName)
	if err != nil {
		return nil, fmt.Errorf("failed to list compose project containers: %w", err)
	}

	containersByNamePrefix, err := s.GetContainersByNamePrefix(ctx, appName+"-")
	if err != nil {
		return nil, fmt.Errorf("failed to list containers by name prefix: %w", err)
	}

	containerByID := make(map[string]types.Container)
	for _, c := range containersByAppLabel {
		containerByID[c.ID] = c
	}
	for _, c := range containersByComposeProject {
		containerByID[c.ID] = c
	}
	for _, c := range containersByNamePrefix {
		containerByID[c.ID] = c
	}

	if len(containerByID) == 0 {
		return []string{fmt.Sprintf("no containers found for app %s", appName)}, nil
	}

	issues := []string{}
	for _, c := range containerByID {
		inspect, err := s.client.ContainerInspect(ctx, c.ID)
		if err != nil {
			issues = append(issues, fmt.Sprintf("failed to inspect container %s: %v", c.ID[:12], err))
			continue
		}

		containerName := strings.TrimPrefix(inspect.Name, "/")
		if containerName == "" {
			containerName = c.ID[:12]
		}

		if inspect.State == nil {
			issues = append(issues, fmt.Sprintf("container %s has empty runtime state", containerName))
			continue
		}

		if !inspect.State.Running {
			issues = append(issues, fmt.Sprintf(
				"container %s is not running (status=%s exit_code=%d error=%s)",
				containerName,
				inspect.State.Status,
				inspect.State.ExitCode,
				inspect.State.Error,
			))
			continue
		}

		if inspect.State.Health != nil {
			healthStatus := inspect.State.Health.Status
			if healthStatus != "" && healthStatus != "healthy" {
				issues = append(issues, fmt.Sprintf("container %s health is %s", containerName, healthStatus))
			}
		}
	}

	sort.Strings(issues)
	return issues, nil
}

// GenerateTraefikLabels generates Traefik labels for a container
func GenerateTraefikLabels(appName string, domains []string, port int, enableTLS bool) map[string]string {
	labels := map[string]string{
		"traefik.enable": "true",
	}

	// If no domains are specified, skip creating routing rules
	if len(domains) == 0 {
		labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", appName)] = fmt.Sprintf("%d", port)
		return labels
	}

	// Build Host() rule for multiple domains
	// Format: Host(`domain1.com`) || Host(`domain2.com`)
	var hostRules []string
	for _, domain := range domains {
		if domain != "" {
			hostRules = append(hostRules, fmt.Sprintf("Host(`%s`)", domain))
		}
	}
	
	// Join rules with OR operator
	rule := ""
	if len(hostRules) > 0 {
		rule = hostRules[0]
		for i := 1; i < len(hostRules); i++ {
			rule += " || " + hostRules[i]
		}
	}

	if rule != "" {
		labels[fmt.Sprintf("traefik.http.routers.%s.rule", appName)] = rule
	}
	
	labels[fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", appName)] = fmt.Sprintf("%d", port)

	// Enable TLS if any domain is specified
	if enableTLS && len(domains) > 0 {
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", appName)] = "websecure"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls", appName)] = "true"
		labels[fmt.Sprintf("traefik.http.routers.%s.tls.certresolver", appName)] = "letsencrypt"
	} else {
		labels[fmt.Sprintf("traefik.http.routers.%s.entrypoints", appName)] = "web"
	}

	return labels
}

// CreateContainerConfig creates a container configuration with Traefik labels
func CreateContainerConfig(imageName string, envVars map[string]string, appName string, domains []string, port int, networks []string) (*container.Config, *container.HostConfig, *network.NetworkingConfig) {
	// Convert env vars to Docker format
	env := []string{}
	for key, value := range envVars {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Generate Traefik labels
	labels := GenerateTraefikLabels(appName, domains, port, len(domains) > 0)
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
