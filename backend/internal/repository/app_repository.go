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
			name, deployment_type, build_type, status, domains, env_vars,
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
		app.Domains, app.EnvVars, app.GitHubRepo, app.GitHubBranch,
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
		SELECT id, name, deployment_type, build_type, status, domains, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at, last_deployed_at
		FROM apps
		WHERE name = ?
	`

	app := &models.App{}
	err := r.db.QueryRow(query, name).Scan(
		&app.ID, &app.Name, &app.DeploymentType, &app.BuildType,
		&app.Status, &app.Domains, &app.EnvVars, &app.GitHubRepo,
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
		SELECT id, name, deployment_type, build_type, status, domains, env_vars,
			github_repo, github_branch, github_subdir, dockerfile_path,
			compose_path, port, created_at, updated_at, last_deployed_at
		FROM apps
		WHERE id = ?
	`

	app := &models.App{}
	err := r.db.QueryRow(query, id).Scan(
		&app.ID, &app.Name, &app.DeploymentType, &app.BuildType,
		&app.Status, &app.Domains, &app.EnvVars, &app.GitHubRepo,
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
		SELECT id, name, deployment_type, build_type, status, domains, env_vars,
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
			&app.Status, &app.Domains, &app.EnvVars, &app.GitHubRepo,
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
			deployment_type = ?, build_type = ?, status = ?, domains = ?,
			env_vars = ?, github_repo = ?, github_branch = ?, github_subdir = ?,
			dockerfile_path = ?, compose_path = ?, port = ?, updated_at = ?,
			last_deployed_at = ?
		WHERE id = ?
	`

	app.UpdatedAt = time.Now()

	_, err := r.db.Exec(
		query,
		app.DeploymentType, app.BuildType, app.Status, app.Domains,
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

// UpsertAppBackupSchedule creates or updates the backup schedule for one app.
func (r *AppRepository) UpsertAppBackupSchedule(schedule *models.AppBackupSchedule) error {
	query := `
		INSERT INTO app_backup_schedules (
			app_id, enabled, interval_minutes, upload_to_s3,
			last_backup_id, last_run_at, next_run_at, last_status, last_error,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(app_id) DO UPDATE SET
			enabled = excluded.enabled,
			interval_minutes = excluded.interval_minutes,
			upload_to_s3 = excluded.upload_to_s3,
			last_backup_id = excluded.last_backup_id,
			last_run_at = excluded.last_run_at,
			next_run_at = excluded.next_run_at,
			last_status = excluded.last_status,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at
	`

	now := time.Now()
	schedule.UpdatedAt = now
	if schedule.CreatedAt.IsZero() {
		schedule.CreatedAt = now
	}

	_, err := r.db.Exec(
		query,
		schedule.AppID,
		schedule.Enabled,
		schedule.IntervalMinutes,
		schedule.UploadToS3,
		schedule.LastBackupID,
		schedule.LastRunAt,
		schedule.NextRunAt,
		schedule.LastStatus,
		schedule.LastError,
		schedule.CreatedAt,
		schedule.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert backup schedule: %w", err)
	}

	return nil
}

// GetAppBackupSchedule gets the backup schedule for one app.
func (r *AppRepository) GetAppBackupSchedule(appID int64) (*models.AppBackupSchedule, error) {
	query := `
		SELECT id, app_id, enabled, interval_minutes, upload_to_s3,
			last_backup_id, last_run_at, next_run_at, last_status, last_error,
			created_at, updated_at
		FROM app_backup_schedules
		WHERE app_id = ?
	`

	schedule := &models.AppBackupSchedule{}
	err := r.db.QueryRow(query, appID).Scan(
		&schedule.ID,
		&schedule.AppID,
		&schedule.Enabled,
		&schedule.IntervalMinutes,
		&schedule.UploadToS3,
		&schedule.LastBackupID,
		&schedule.LastRunAt,
		&schedule.NextRunAt,
		&schedule.LastStatus,
		&schedule.LastError,
		&schedule.CreatedAt,
		&schedule.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get backup schedule: %w", err)
	}

	return schedule, nil
}

// ListDueAppBackupSchedules returns enabled schedules that are due to run.
func (r *AppRepository) ListDueAppBackupSchedules(now time.Time, limit int) ([]*models.AppBackupSchedule, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, app_id, enabled, interval_minutes, upload_to_s3,
			last_backup_id, last_run_at, next_run_at, last_status, last_error,
			created_at, updated_at
		FROM app_backup_schedules
		WHERE enabled = 1 AND next_run_at IS NOT NULL AND next_run_at <= ?
		ORDER BY next_run_at ASC
		LIMIT ?
	`

	rows, err := r.db.Query(query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list due backup schedules: %w", err)
	}
	defer rows.Close()

	schedules := []*models.AppBackupSchedule{}
	for rows.Next() {
		schedule := &models.AppBackupSchedule{}
		if err := rows.Scan(
			&schedule.ID,
			&schedule.AppID,
			&schedule.Enabled,
			&schedule.IntervalMinutes,
			&schedule.UploadToS3,
			&schedule.LastBackupID,
			&schedule.LastRunAt,
			&schedule.NextRunAt,
			&schedule.LastStatus,
			&schedule.LastError,
			&schedule.CreatedAt,
			&schedule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan backup schedule: %w", err)
		}
		schedules = append(schedules, schedule)
	}

	return schedules, nil
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

// CreateAppBackup creates an app backup entry
func (r *AppRepository) CreateAppBackup(backup *models.AppBackup) error {
	query := `
		INSERT INTO app_backups (backup_id, app_id, status, path, size_bytes, log, started_at, completed_at, s3_path, uploaded_to_s3, s3_uploaded_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	backup.StartedAt = time.Now()

	result, err := r.db.Exec(
		query,
		backup.BackupID,
		backup.AppID,
		backup.Status,
		backup.Path,
		backup.SizeBytes,
		backup.Log,
		backup.StartedAt,
		backup.CompletedAt,
		backup.S3Path,
		backup.UploadedToS3,
		backup.S3UploadedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create app backup: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get app backup last insert id: %w", err)
	}

	backup.ID = id
	return nil
}

// UpdateAppBackup updates an app backup entry
func (r *AppRepository) UpdateAppBackup(backup *models.AppBackup) error {
	query := `
		UPDATE app_backups
		SET status = ?, path = ?, size_bytes = ?, log = ?, completed_at = ?, s3_path = ?, uploaded_to_s3 = ?, s3_uploaded_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(
		query,
		backup.Status,
		backup.Path,
		backup.SizeBytes,
		backup.Log,
		backup.CompletedAt,
		backup.S3Path,
		backup.UploadedToS3,
		backup.S3UploadedAt,
		backup.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update app backup: %w", err)
	}

	return nil
}

// GetAppBackupByBackupID gets an app backup by app id and backup id
func (r *AppRepository) GetAppBackupByBackupID(appID int64, backupID string) (*models.AppBackup, error) {
	query := `
		SELECT id, backup_id, app_id, status, path, size_bytes, log, started_at, completed_at, s3_path, uploaded_to_s3, s3_uploaded_at
		FROM app_backups
		WHERE app_id = ? AND backup_id = ?
	`

	backup := &models.AppBackup{}
	err := r.db.QueryRow(query, appID, backupID).Scan(
		&backup.ID,
		&backup.BackupID,
		&backup.AppID,
		&backup.Status,
		&backup.Path,
		&backup.SizeBytes,
		&backup.Log,
		&backup.StartedAt,
		&backup.CompletedAt,
		&backup.S3Path,
		&backup.UploadedToS3,
		&backup.S3UploadedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get app backup: %w", err)
	}

	return backup, nil
}

// ListAppBackups lists app backups for one app
func (r *AppRepository) ListAppBackups(appID int64, limit int) ([]*models.AppBackup, error) {
	if limit <= 0 {
		limit = 20
	}

	query := `
		SELECT id, backup_id, app_id, status, path, size_bytes, log, started_at, completed_at, s3_path, uploaded_to_s3, s3_uploaded_at
		FROM app_backups
		WHERE app_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, appID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list app backups: %w", err)
	}
	defer rows.Close()

	backups := []*models.AppBackup{}
	for rows.Next() {
		backup := &models.AppBackup{}
		err := rows.Scan(
			&backup.ID,
			&backup.BackupID,
			&backup.AppID,
			&backup.Status,
			&backup.Path,
			&backup.SizeBytes,
			&backup.Log,
			&backup.StartedAt,
			&backup.CompletedAt,
			&backup.S3Path,
			&backup.UploadedToS3,
			&backup.S3UploadedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan app backup: %w", err)
		}
		backups = append(backups, backup)
	}

	return backups, nil
}

// DeleteAppBackup deletes an app backup record
func (r *AppRepository) DeleteAppBackup(appID int64, backupID string) error {
	query := `DELETE FROM app_backups WHERE app_id = ? AND backup_id = ?`
	_, err := r.db.Exec(query, appID, backupID)
	if err != nil {
		return fmt.Errorf("failed to delete app backup: %w", err)
	}
	return nil
}

// CreateAppRestore creates an app restore entry
func (r *AppRepository) CreateAppRestore(restore *models.AppRestore) error {
	query := `
		INSERT INTO app_restores (restore_id, app_id, backup_id, status, log, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	restore.StartedAt = time.Now()

	result, err := r.db.Exec(
		query,
		restore.RestoreID,
		restore.AppID,
		restore.BackupID,
		restore.Status,
		restore.Log,
		restore.StartedAt,
		restore.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create app restore: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get app restore last insert id: %w", err)
	}

	restore.ID = id
	return nil
}

// UpdateAppRestore updates an app restore entry
func (r *AppRepository) UpdateAppRestore(restore *models.AppRestore) error {
	query := `
		UPDATE app_restores
		SET status = ?, log = ?, completed_at = ?
		WHERE id = ?
	`

	_, err := r.db.Exec(
		query,
		restore.Status,
		restore.Log,
		restore.CompletedAt,
		restore.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update app restore: %w", err)
	}

	return nil
}

// GetAppRestoreByRestoreID gets one restore by app id and restore id
func (r *AppRepository) GetAppRestoreByRestoreID(appID int64, restoreID string) (*models.AppRestore, error) {
	query := `
		SELECT id, restore_id, app_id, backup_id, status, log, started_at, completed_at
		FROM app_restores
		WHERE app_id = ? AND restore_id = ?
	`

	restore := &models.AppRestore{}
	err := r.db.QueryRow(query, appID, restoreID).Scan(
		&restore.ID,
		&restore.RestoreID,
		&restore.AppID,
		&restore.BackupID,
		&restore.Status,
		&restore.Log,
		&restore.StartedAt,
		&restore.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get app restore: %w", err)
	}

	return restore, nil
}

// GetLatestAppRestoreByBackupID gets the latest restore by app id and backup id.
func (r *AppRepository) GetLatestAppRestoreByBackupID(appID int64, backupID string) (*models.AppRestore, error) {
	query := `
		SELECT id, restore_id, app_id, backup_id, status, log, started_at, completed_at
		FROM app_restores
		WHERE app_id = ? AND backup_id = ?
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`

	restore := &models.AppRestore{}
	err := r.db.QueryRow(query, appID, backupID).Scan(
		&restore.ID,
		&restore.RestoreID,
		&restore.AppID,
		&restore.BackupID,
		&restore.Status,
		&restore.Log,
		&restore.StartedAt,
		&restore.CompletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get latest app restore by backup id: %w", err)
	}

	return restore, nil
}
