package repository

import (
	"atmosphere/internal/models"
	"database/sql"
	"fmt"
	"time"
)

// AppRepository handles database operations for apps
type AppRepository struct {
	db *sql.DB
}

// NewAppRepository creates a new app repository
func NewAppRepository(db *sql.DB) *AppRepository {
	return &AppRepository{db: db}
}

// Create creates a new app in the database
func (r *AppRepository) Create(app *models.App) error {
	query := `
		INSERT INTO apps (
			name, deployment_type, build_type, status, domain, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now()
	app.CreatedAt = now
	app.UpdatedAt = now

	result, err := r.db.Exec(
		query,
		app.Name, app.DeploymentType, app.BuildType, app.Status,
		app.Domain, app.EnvVars, app.GitHubRepo, app.GitHubBranch,
		app.GitHubSubdir, app.DockerfilePath, app.ComposePath,
		app.Port, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	app.ID = id
	return nil
}

// GetByName retrieves an app by name
func (r *AppRepository) GetByName(name string) (*models.App, error) {
	query := `
		SELECT id, name, deployment_type, build_type, status, domain, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at, last_deployed_at
		FROM apps
		WHERE name = ?
	`

	app := &models.App{}
	err := r.db.QueryRow(query, name).Scan(
		&app.ID, &app.Name, &app.DeploymentType, &app.BuildType,
		&app.Status, &app.Domain, &app.EnvVars, &app.GitHubRepo,
		&app.GitHubBranch, &app.GitHubSubdir, &app.DockerfilePath,
		&app.ComposePath, &app.Port, &app.CreatedAt, &app.UpdatedAt,
		&app.LastDeployedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	return app, nil
}

// GetByID retrieves an app by ID
func (r *AppRepository) GetByID(id int64) (*models.App, error) {
	query := `
		SELECT id, name, deployment_type, build_type, status, domain, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at, last_deployed_at
		FROM apps
		WHERE id = ?
	`

	app := &models.App{}
	err := r.db.QueryRow(query, id).Scan(
		&app.ID, &app.Name, &app.DeploymentType, &app.BuildType,
		&app.Status, &app.Domain, &app.EnvVars, &app.GitHubRepo,
		&app.GitHubBranch, &app.GitHubSubdir, &app.DockerfilePath,
		&app.ComposePath, &app.Port, &app.CreatedAt, &app.UpdatedAt,
		&app.LastDeployedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get app: %w", err)
	}

	return app, nil
}

// List retrieves all apps
func (r *AppRepository) List() ([]*models.App, error) {
	query := `
		SELECT id, name, deployment_type, build_type, status, domain, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at, last_deployed_at
		FROM apps
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list apps: %w", err)
	}
	defer rows.Close()

	apps := []*models.App{}
	for rows.Next() {
		app := &models.App{}
		err := rows.Scan(
			&app.ID, &app.Name, &app.DeploymentType, &app.BuildType,
			&app.Status, &app.Domain, &app.EnvVars, &app.GitHubRepo,
			&app.GitHubBranch, &app.GitHubSubdir, &app.DockerfilePath,
			&app.ComposePath, &app.Port, &app.CreatedAt, &app.UpdatedAt,
			&app.LastDeployedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan app: %w", err)
		}
		apps = append(apps, app)
	}

	return apps, nil
}

// Update updates an existing app
func (r *AppRepository) Update(app *models.App) error {
	query := `
		UPDATE apps SET
			deployment_type = ?, build_type = ?, status = ?, domain = ?,
			env_vars = ?, github_repo = ?, github_branch = ?, github_subdir = ?,
			dockerfile_path = ?, compose_path = ?, port = ?, updated_at = ?,
			last_deployed_at = ?
		WHERE id = ?
	`

	app.UpdatedAt = time.Now()

	_, err := r.db.Exec(
		query,
		app.DeploymentType, app.BuildType, app.Status, app.Domain,
		app.EnvVars, app.GitHubRepo, app.GitHubBranch, app.GitHubSubdir,
		app.DockerfilePath, app.ComposePath, app.Port, app.UpdatedAt,
		app.LastDeployedAt, app.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	return nil
}

// Delete deletes an app
func (r *AppRepository) Delete(id int64) error {
	query := `DELETE FROM apps WHERE id = ?`
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}
	return nil
}

// CreateDeploymentLog creates a deployment log entry
func (r *AppRepository) CreateDeploymentLog(log *models.DeploymentLog) error {
	query := `
		INSERT INTO deployment_logs (app_id, status, log, started_at, ended_at)
		VALUES (?, ?, ?, ?, ?)
	`

	log.StartedAt = time.Now()

	result, err := r.db.Exec(
		query,
		log.AppID, log.Status, log.Log, log.StartedAt, log.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create deployment log: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	log.ID = id
	return nil
}

// UpdateDeploymentLog updates a deployment log entry
func (r *AppRepository) UpdateDeploymentLog(log *models.DeploymentLog) error {
	query := `
		UPDATE deployment_logs SET
			status = ?, log = ?, ended_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(query, log.Status, log.Log, log.EndedAt, log.ID)
	if err != nil {
		return fmt.Errorf("failed to update deployment log: %w", err)
	}

	return nil
}

// GetDeploymentLogs retrieves deployment logs for an app
func (r *AppRepository) GetDeploymentLogs(appID int64, limit int) ([]*models.DeploymentLog, error) {
	query := `
		SELECT id, app_id, status, log, started_at, ended_at
		FROM deployment_logs
		WHERE app_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, appID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment logs: %w", err)
	}
	defer rows.Close()

	logs := []*models.DeploymentLog{}
	for rows.Next() {
		log := &models.DeploymentLog{}
		err := rows.Scan(
			&log.ID, &log.AppID, &log.Status, &log.Log,
			&log.StartedAt, &log.EndedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment log: %w", err)
		}
		logs = append(logs, log)
	}

	return logs, nil
}
