package services

import (
	"archive/tar"
	"atmosphere/internal/config"
	"atmosphere/internal/models"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// DeploymentService handles deployment operations
type DeploymentService struct {
	cfg           *config.Config
	dockerService *DockerService
}

// NewDeploymentService creates a new deployment service
func NewDeploymentService(cfg *config.Config, dockerService *DockerService) *DeploymentService {
	return &DeploymentService{
		cfg:           cfg,
		dockerService: dockerService,
	}
}

// Deploy deploys an application
func (s *DeploymentService) Deploy(ctx context.Context, app *models.App) (string, error) {
	logOutput := &strings.Builder{}

	// Prepare workspace
	workspaceDir := s.getWorkspaceDir(app.Name)
	logOutput.WriteString(fmt.Sprintf("[%s] Preparing workspace: %s\n", time.Now().Format("15:04:05"), workspaceDir))

	// For GitHub deployments, clone/pull the repository
	if app.DeploymentType == "github" {
		if err := s.prepareGitHubDeployment(app, logOutput); err != nil {
			return logOutput.String(), fmt.Errorf("failed to prepare GitHub deployment: %w", err)
		}
	}

	// Determine build directory
	buildDir := workspaceDir
	if app.GitHubSubdir != "" {
		buildDir = filepath.Join(workspaceDir, app.GitHubSubdir)
	}

	// Detect or use specified build files
	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	composePath := ""

	if app.BuildType == "compose" {
		// Prefer explicitly specified compose file path
		logOutput.WriteString(fmt.Sprintf("[%s] DEBUG: app.ComposePath = '%s'\n", time.Now().Format("15:04:05"), app.ComposePath))
		if app.ComposePath != "" {
			composePath = filepath.Join(buildDir, app.ComposePath)
			logOutput.WriteString(fmt.Sprintf("[%s] DEBUG: Using specified compose path: %s\n", time.Now().Format("15:04:05"), composePath))
		} else {
			composePath = s.DetectComposeFile(buildDir)
			logOutput.WriteString(fmt.Sprintf("[%s] DEBUG: Auto-detected compose path: %s\n", time.Now().Format("15:04:05"), composePath))
		}
		if composePath == "" {
			return logOutput.String(), fmt.Errorf("no docker-compose file found")
		}
		logOutput.WriteString(fmt.Sprintf("[%s] Using compose file: %s\n", time.Now().Format("15:04:05"), composePath))
	} else {
		if app.DockerfilePath != "" {
			dockerfilePath = filepath.Join(buildDir, app.DockerfilePath)
		}
		logOutput.WriteString(fmt.Sprintf("[%s] Using Dockerfile: %s\n", time.Now().Format("15:04:05"), dockerfilePath))
	}

	// Stop existing containers
	if err := s.stopExistingContainers(ctx, app, logOutput); err != nil {
		logOutput.WriteString(fmt.Sprintf("[%s] Warning: failed to stop existing containers: %v\n", time.Now().Format("15:04:05"), err))
	}

	// Build and deploy based on build type
	if app.BuildType == "compose" {
		if err := s.deployCompose(ctx, app, composePath, buildDir, logOutput); err != nil {
			return logOutput.String(), err
		}
	} else {
		if err := s.deployDockerfile(ctx, app, dockerfilePath, buildDir, logOutput); err != nil {
			return logOutput.String(), err
		}
	}

	logOutput.WriteString(fmt.Sprintf("[%s] Deployment successful!\n", time.Now().Format("15:04:05")))
	return logOutput.String(), nil
}

// prepareGitHubDeployment clones or pulls a GitHub repository
func (s *DeploymentService) prepareGitHubDeployment(app *models.App, logOutput *strings.Builder) error {
	workspaceDir := s.getWorkspaceDir(app.Name)
	keyPath := s.getDeploymentKeyPath(app.Name)

	logOutput.WriteString(fmt.Sprintf("[%s] Preparing GitHub deployment from %s\n", time.Now().Format("15:04:05"), app.GitHubRepo))

	// Check if workspace already exists
	if _, err := os.Stat(filepath.Join(workspaceDir, ".git")); err == nil {
		// Repository exists, pull latest changes
		logOutput.WriteString(fmt.Sprintf("[%s] Repository exists, pulling latest changes\n", time.Now().Format("15:04:05")))
		return s.gitPull(workspaceDir, keyPath, app.GitHubBranch, logOutput)
	}

	// Clone repository
	logOutput.WriteString(fmt.Sprintf("[%s] Cloning repository\n", time.Now().Format("15:04:05")))
	return s.gitClone(app.GitHubRepo, workspaceDir, keyPath, app.GitHubBranch, logOutput)
}

// gitClone clones a Git repository
func (s *DeploymentService) gitClone(repo, dest, keyPath, branch string, logOutput *strings.Builder) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		logOutput.WriteString(fmt.Sprintf("[%s] ERROR: SSH key file not found at %s\n", time.Now().Format("15:04:05"), keyPath))
		return fmt.Errorf("SSH key file not found at %s", keyPath)
	}
	
	// Log key file info
	keyInfo, _ := os.Stat(keyPath)
	if keyInfo != nil {
		logOutput.WriteString(fmt.Sprintf("[%s] Using SSH key: %s (permissions: %s)\n", time.Now().Format("15:04:05"), keyPath, keyInfo.Mode()))
	}

	// Set up SSH command with deployment key
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", keyPath)
	
	// Clone command
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, repo, dest)

	cmd := exec.Command("git", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))

	output, err := cmd.CombinedOutput()
	logOutput.Write(output)

	if err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// gitPull pulls latest changes from Git repository
func (s *DeploymentService) gitPull(repoDir, keyPath, branch string, logOutput *strings.Builder) error {
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", keyPath)

	// Fetch
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
	output, err := cmd.CombinedOutput()
	logOutput.Write(output)
	if err != nil {
		return fmt.Errorf("git fetch failed: %w", err)
	}

	// Checkout branch
	if branch != "" {
		cmd = exec.Command("git", "checkout", branch)
		cmd.Dir = repoDir
		output, err = cmd.CombinedOutput()
		logOutput.Write(output)
		if err != nil {
			return fmt.Errorf("git checkout failed: %w", err)
		}
	}

	// Pull
	cmd = exec.Command("git", "pull", "origin", branch)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", sshCmd))
	output, err = cmd.CombinedOutput()
	logOutput.Write(output)
	if err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}

// DetectComposeFile detects the compose file in a directory
func (s *DeploymentService) DetectComposeFile(dir string) string {
	composeFiles := []string{
		"docker-compose.yml",
		"docker-compose.yaml",
		"compose.yml",
		"compose.yaml",
	}

	for _, file := range composeFiles {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// deployDockerfile builds and deploys a Dockerfile-based app
func (s *DeploymentService) deployDockerfile(ctx context.Context, app *models.App, dockerfilePath, buildDir string, logOutput *strings.Builder) error {
	logOutput.WriteString(fmt.Sprintf("[%s] Building Docker image\n", time.Now().Format("15:04:05")))

	// Create tar archive of build context
	tarBuffer := &bytes.Buffer{}
	if err := createTarArchive(buildDir, tarBuffer); err != nil {
		return fmt.Errorf("failed to create tar archive: %w", err)
	}

	// Build image
	imageName := fmt.Sprintf("atmosphere-%s:latest", app.Name)
	buildOutput, err := s.dockerService.BuildImage(ctx, tarBuffer, imageName)
	if err != nil {
		return fmt.Errorf("failed to build image: %w", err)
	}
	logOutput.WriteString(buildOutput)

	// Determine port
	port := app.Port
	if port == 0 {
		port = 8080 // Default port
	}

	// Create and start container
	logOutput.WriteString(fmt.Sprintf("[%s] Creating container\n", time.Now().Format("15:04:05")))

	networks := []string{s.cfg.DockerNetwork, s.cfg.TraefikNetwork}
	containerConfig, hostConfig, networkConfig := CreateContainerConfig(
		imageName,
		app.EnvVars,
		app.Name,
		app.Domains,
		port,
		networks,
	)

	containerName := fmt.Sprintf("atmosphere-%s", app.Name)
	containerID, err := s.dockerService.CreateContainer(ctx, containerConfig, hostConfig, networkConfig, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	logOutput.WriteString(fmt.Sprintf("[%s] Starting container: %s\n", time.Now().Format("15:04:05"), containerID[:12]))

	if err := s.dockerService.StartContainer(ctx, containerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	return nil
}

// deployCompose deploys a docker-compose based app
func (s *DeploymentService) deployCompose(ctx context.Context, app *models.App, composePath, buildDir string, logOutput *strings.Builder) error {
	logOutput.WriteString(fmt.Sprintf("[%s] Deploying with docker-compose\n", time.Now().Format("15:04:05")))

	// Create .env file with app environment variables
	envFilePath := filepath.Join(buildDir, ".env.atmosphere")
	if err := s.createEnvFile(envFilePath, app.EnvVars); err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}

	// Set project name
	projectName := fmt.Sprintf("atmosphere-%s", app.Name)

	// Build compose file arguments
	// If using a specific compose file (not the default), include base docker-compose.yml first
	composeArgs := []string{"compose"}
	baseCompose := filepath.Join(buildDir, "docker-compose.yml")
	if composePath != baseCompose && fileExists(baseCompose) {
		// Using override file - include base first
		composeArgs = append(composeArgs, "-f", baseCompose, "-f", composePath)
		logOutput.WriteString(fmt.Sprintf("[%s] Using base compose file: %s\n", time.Now().Format("15:04:05"), baseCompose))
		logOutput.WriteString(fmt.Sprintf("[%s] Using override compose file: %s\n", time.Now().Format("15:04:05"), composePath))
	} else {
		// Using standalone compose file
		composeArgs = append(composeArgs, "-f", composePath)
	}
	composeArgs = append(composeArgs, "-p", projectName)

	// Run docker compose build
	buildArgs := append(composeArgs, "build")
	cmd := s.CreateComposeCommand(ctx, buildDir, buildArgs, app)
	output, err := cmd.CombinedOutput()
	logOutput.Write(output)
	if err != nil {
		return fmt.Errorf("docker compose build failed: %w", err)
	}

	// Run docker compose up
	logOutput.WriteString(fmt.Sprintf("[%s] Starting services\n", time.Now().Format("15:04:05")))
	upArgs := append(composeArgs, "up", "-d")
	cmd = s.CreateComposeCommand(ctx, buildDir, upArgs, app)
	output, err = cmd.CombinedOutput()
	logOutput.Write(output)
	if err != nil {
		return fmt.Errorf("docker compose up failed: %w", err)
	}

	return nil
}

// stopExistingContainers stops containers for an app
func (s *DeploymentService) stopExistingContainers(ctx context.Context, app *models.App, logOutput *strings.Builder) error {
	containers, err := s.dockerService.GetContainersByLabel(ctx, "atmosphere.app", app.Name)
	if err != nil {
		return err
	}

	for _, container := range containers {
		logOutput.WriteString(fmt.Sprintf("[%s] Stopping container: %s\n", time.Now().Format("15:04:05"), container.ID[:12]))
		if err := s.dockerService.StopContainer(ctx, container.ID); err != nil {
			logOutput.WriteString(fmt.Sprintf("[%s] Warning: failed to stop container %s: %v\n", time.Now().Format("15:04:05"), container.ID[:12], err))
		}
		if err := s.dockerService.RemoveContainer(ctx, container.ID); err != nil {
			logOutput.WriteString(fmt.Sprintf("[%s] Warning: failed to remove container %s: %v\n", time.Now().Format("15:04:05"), container.ID[:12], err))
		}
	}

	return nil
}

// Stop stops an application
func (s *DeploymentService) Stop(ctx context.Context, app *models.App) error {
	containers, err := s.dockerService.GetContainersByLabel(ctx, "atmosphere.app", app.Name)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := s.dockerService.StopContainer(ctx, container.ID); err != nil {
			return fmt.Errorf("failed to stop container %s: %w", container.ID[:12], err)
		}
	}

	return nil
}

// Start starts an application
func (s *DeploymentService) Start(ctx context.Context, app *models.App) error {
	containers, err := s.dockerService.GetContainersByLabel(ctx, "atmosphere.app", app.Name)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := s.dockerService.StartContainer(ctx, container.ID); err != nil {
			return fmt.Errorf("failed to start container %s: %w", container.ID[:12], err)
		}
	}

	return nil
}

// Remove removes an application and its containers
func (s *DeploymentService) Remove(ctx context.Context, app *models.App) error {
	// Stop and remove containers
	containers, err := s.dockerService.GetContainersByLabel(ctx, "atmosphere.app", app.Name)
	if err != nil {
		return err
	}

	for _, container := range containers {
		if err := s.dockerService.StopContainer(ctx, container.ID); err != nil {
			// Continue even if stop fails
		}
		if err := s.dockerService.RemoveContainer(ctx, container.ID); err != nil {
			return fmt.Errorf("failed to remove container %s: %w", container.ID[:12], err)
		}
	}

	// Remove workspace directory
	workspaceDir := s.getWorkspaceDir(app.Name)
	if err := os.RemoveAll(workspaceDir); err != nil {
		return fmt.Errorf("failed to remove workspace: %w", err)
	}

	// Remove deployment key
	keyPath := s.getDeploymentKeyPath(app.Name)
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove deployment key: %w", err)
	}

	return nil
}

// Helper functions

func (s *DeploymentService) getWorkspaceDir(appName string) string {
	return filepath.Join(s.cfg.WorkspacesDir, appName)
}

func (s *DeploymentService) getDeploymentKeyPath(appName string) string {
	return filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", appName))
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (s *DeploymentService) createEnvFile(path string, envVars map[string]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	for key, value := range envVars {
		if _, err := f.WriteString(fmt.Sprintf("%s=%s\n", key, value)); err != nil {
			return err
		}
	}

	return nil
}

// CreateComposeCommand creates a docker compose command with environment variables
func (s *DeploymentService) CreateComposeCommand(ctx context.Context, workDir string, args []string, app *models.App) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = workDir
	
	// Build environment map to handle precedence correctly
	// Precedence (lowest to highest): container.env < .env < Atmosphere variables
	envMap := make(map[string]string)
	
	// Start with system environment
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	
	// Load environment variables from env files (Docker Compose standard precedence)
	// Load container.env first (lower priority)
	if containerEnv, err := godotenv.Read(filepath.Join(workDir, "container.env")); err == nil {
		for key, value := range containerEnv {
			envMap[key] = value
		}
	}
	
	// Load .env second (higher priority - matches Docker Compose behavior)
	if dotEnv, err := godotenv.Read(filepath.Join(workDir, ".env")); err == nil {
		for key, value := range dotEnv {
			envMap[key] = value
		}
	}
	
	// Add Atmosphere-specific variables (highest priority)
	envMap["ATMOSPHERE_APP"] = app.Name
	envMap["TRAEFIK_NETWORK"] = s.cfg.TraefikNetwork
	// Set DOMAIN to first domain for backward compatibility, DOMAINS as comma-separated list
	if len(app.Domains) > 0 {
		envMap["DOMAIN"] = app.Domains[0]
		envMap["DOMAINS"] = ""
		for i, domain := range app.Domains {
			if i > 0 {
				envMap["DOMAINS"] += ","
			}
			envMap["DOMAINS"] += domain
		}
	} else {
		envMap["DOMAIN"] = ""
		envMap["DOMAINS"] = ""
	}
	
	// Convert map to environment array
	env := make([]string, 0, len(envMap))
	for key, value := range envMap {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	
	cmd.Env = env
	return cmd
}

// createTarArchive creates a tar archive from a directory
func createTarArchive(srcDir string, w io.Writer) error {
	tw := tar.NewWriter(w)
	defer tw.Close()

	return filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip .git directory
		if fi.IsDir() && fi.Name() == ".git" {
			return filepath.SkipDir
		}

		// Create tar header
		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		// Update header name to be relative to srcDir
		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		// Write header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If not a regular file, we're done
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Open and copy file content
		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return err
		}

		return nil
	})
}
