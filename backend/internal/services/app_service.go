package services

import (
	"atmosphere/internal/config"
	"atmosphere/internal/models"
	"atmosphere/internal/repository"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// AppService handles app business logic
type AppService struct {
	repo              *repository.AppRepository
	cfg               *config.Config
	deploymentService *DeploymentService
}

// NewAppService creates a new app service
func NewAppService(repo *repository.AppRepository, cfg *config.Config, deploymentService *DeploymentService) *AppService {
	return &AppService{
		repo:              repo,
		cfg:               cfg,
		deploymentService: deploymentService,
	}
}

// CreateApp creates a new app
func (s *AppService) CreateApp(req *models.CreateAppRequest) (*models.App, error) {
	// Validate request
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if app already exists
	existing, err := s.repo.GetByName(req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing app: %w", err)
	}
	if existing != nil {
		return nil, fmt.Errorf("app with name %s already exists", req.Name)
	}

	// Create workspace directory
	workspaceDir := filepath.Join(s.cfg.WorkspacesDir, req.Name)
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// For GitHub deployments, save deployment key
	if req.DeploymentType == "github" && req.DeploymentKey != "" {
		if err := s.saveDeploymentKey(req.Name, req.DeploymentKey); err != nil {
			return nil, fmt.Errorf("failed to save deployment key: %w", err)
		}
	}

	// Set default port if not specified
	port := req.Port
	if port == 0 {
		port = 8080
	}

	// Create app model
	app := &models.App{
		Name:           req.Name,
		DeploymentType: req.DeploymentType,
		BuildType:      req.BuildType,
		Status:         "stopped",
		Domain:         req.Domain,
		EnvVars:        req.EnvVars,
		GitHubRepo:     req.GitHubRepo,
		GitHubBranch:   req.GitHubBranch,
		GitHubSubdir:   req.GitHubSubdir,
		DockerfilePath: req.DockerfilePath,
		ComposePath:    req.ComposePath,
		Port:           port,
	}

	if app.EnvVars == nil {
		app.EnvVars = make(models.EnvVars)
	}

	// Save to database
	if err := s.repo.Create(app); err != nil {
		return nil, fmt.Errorf("failed to create app: %w", err)
	}

	return app, nil
}

// GetApp retrieves an app by name
func (s *AppService) GetApp(name string) (*models.App, error) {
	app, err := s.repo.GetByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}
	if app == nil {
		return nil, fmt.Errorf("app not found")
	}
	return app, nil
}

// ListApps retrieves all apps
func (s *AppService) ListApps() ([]*models.App, error) {
	apps, err := s.repo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	return apps, nil
}

// UpdateApp updates an existing app
func (s *AppService) UpdateApp(name string, req *models.UpdateAppRequest) (*models.App, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	// Update fields if provided
	if req.Domain != nil {
		app.Domain = *req.Domain
	}
	if req.EnvVars != nil {
		app.EnvVars = *req.EnvVars
	}
	if req.GitHubBranch != nil {
		app.GitHubBranch = *req.GitHubBranch
	}
	if req.GitHubSubdir != nil {
		app.GitHubSubdir = *req.GitHubSubdir
	}
	if req.DockerfilePath != nil {
		app.DockerfilePath = *req.DockerfilePath
	}
	if req.ComposePath != nil {
		app.ComposePath = *req.ComposePath
	}
	if req.Port != nil {
		app.Port = *req.Port
	}

	// Save to database
	if err := s.repo.Update(app); err != nil {
		return nil, fmt.Errorf("failed to update app: %w", err)
	}

	return app, nil
}

// DeleteApp deletes an app
func (s *AppService) DeleteApp(name string) error {
	app, err := s.GetApp(name)
	if err != nil {
		return err
	}

	// Remove containers and resources
	ctx := context.Background()
	if err := s.deploymentService.Remove(ctx, app); err != nil {
		return fmt.Errorf("failed to remove app resources: %w", err)
	}

	// Delete from database
	if err := s.repo.Delete(app.ID); err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}

	return nil
}

// DeployApp deploys an app
func (s *AppService) DeployApp(name string) (*models.DeploymentLog, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	// Update app status to building
	app.Status = "building"
	if err := s.repo.Update(app); err != nil {
		return nil, fmt.Errorf("failed to update app status: %w", err)
	}

	// Create deployment log
	deployLog := &models.DeploymentLog{
		AppID:  app.ID,
		Status: "in_progress",
		Log:    "",
	}
	if err := s.repo.CreateDeploymentLog(deployLog); err != nil {
		return nil, fmt.Errorf("failed to create deployment log: %w", err)
	}

	// Deploy in background
	go func() {
		ctx := context.Background()
		logOutput, err := s.deploymentService.Deploy(ctx, app)

		// Update deployment log
		deployLog.Log = logOutput
		now := time.Now()
		deployLog.EndedAt = &now

		if err != nil {
			deployLog.Status = "failed"
			app.Status = "failed"
			deployLog.Log += fmt.Sprintf("\n\nError: %v", err)
		} else {
			deployLog.Status = "success"
			app.Status = "running"
			now := time.Now()
			app.LastDeployedAt = &now
		}

		// Save deployment log
		_ = s.repo.UpdateDeploymentLog(deployLog)

		// Update app status
		_ = s.repo.Update(app)
	}()

	return deployLog, nil
}

// StopApp stops an app
func (s *AppService) StopApp(name string) error {
	app, err := s.GetApp(name)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := s.deploymentService.Stop(ctx, app); err != nil {
		return fmt.Errorf("failed to stop app: %w", err)
	}

	app.Status = "stopped"
	if err := s.repo.Update(app); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}

	return nil
}

// StartApp starts an app
func (s *AppService) StartApp(name string) error {
	app, err := s.GetApp(name)
	if err != nil {
		return err
	}

	ctx := context.Background()
	if err := s.deploymentService.Start(ctx, app); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	app.Status = "running"
	if err := s.repo.Update(app); err != nil {
		return fmt.Errorf("failed to update app status: %w", err)
	}

	return nil
}

// GetDeploymentLogs retrieves deployment logs for an app
func (s *AppService) GetDeploymentLogs(name string, limit int) ([]*models.DeploymentLog, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}

	logs, err := s.repo.GetDeploymentLogs(app.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment logs: %w", err)
	}

	return logs, nil
}

// UploadFile uploads a file to an app's workspace
func (s *AppService) UploadFile(name, filePath string, content []byte) error {
	app, err := s.GetApp(name)
	if err != nil {
		return err
	}

	if app.DeploymentType != "manual" {
		return fmt.Errorf("file upload only supported for manual deployments")
	}

	// Validate file path to prevent path traversal
	if !isValidFilePath(filePath) {
		return fmt.Errorf("invalid file path")
	}

	// Create full file path
	workspaceDir := filepath.Join(s.cfg.WorkspacesDir, app.Name)
	fullPath := filepath.Join(workspaceDir, filepath.Clean(filePath))

	// Ensure the path is within the workspace
	if !isPathWithin(fullPath, workspaceDir) {
		return fmt.Errorf("file path outside workspace")
	}

	// Create parent directory if needed
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Helper functions

func (s *AppService) validateCreateRequest(req *models.CreateAppRequest) error {
	// Validate app name
	if !isValidAppName(req.Name) {
		return fmt.Errorf("invalid app name: must be lowercase alphanumeric with hyphens, max 32 chars")
	}

	// Validate deployment type
	if req.DeploymentType != "github" && req.DeploymentType != "manual" {
		return fmt.Errorf("deployment_type must be 'github' or 'manual'")
	}

	// Validate build type
	if req.BuildType != "dockerfile" && req.BuildType != "compose" {
		return fmt.Errorf("build_type must be 'dockerfile' or 'compose'")
	}

	// Validate GitHub deployment requirements
	if req.DeploymentType == "github" {
		if req.GitHubRepo == "" {
			return fmt.Errorf("github_repo required for GitHub deployments")
		}
		if req.DeploymentKey == "" {
			return fmt.Errorf("deployment_key required for GitHub deployments")
		}
		if req.GitHubBranch == "" {
			req.GitHubBranch = "main" // Default branch
		}
	}

	// Validate domain format if provided
	if req.Domain != "" && !isValidDomain(req.Domain) {
		return fmt.Errorf("invalid domain format")
	}

	return nil
}

func (s *AppService) saveDeploymentKey(appName, key string) error {
	keyPath := filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", appName))

	// Write key to file with restrictive permissions
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return fmt.Errorf("failed to write deployment key: %w", err)
	}

	return nil
}

// Validation helpers

func isValidAppName(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	match, _ := regexp.MatchString(`^[a-z0-9-]+$`, name)
	return match
}

func isValidDomain(domain string) bool {
	match, _ := regexp.MatchString(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`, domain)
	return match
}

func isValidFilePath(path string) bool {
	// Prevent path traversal
	if filepath.IsAbs(path) {
		return false
	}
	cleanPath := filepath.Clean(path)
	if cleanPath == ".." || filepath.HasPrefix(cleanPath, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}

func isPathWithin(targetPath, basePath string) bool {
	rel, err := filepath.Rel(basePath, targetPath)
	if err != nil {
		return false
	}
	return !filepath.IsAbs(rel) && !filepath.HasPrefix(rel, "..")
}
