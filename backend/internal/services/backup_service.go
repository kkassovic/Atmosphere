package services

import (
	"archive/tar"
	"atmosphere/internal/models"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CreateAppBackup starts an asynchronous backup for one app.
func (s *AppService) CreateAppBackup(name string, uploadToS3 bool) (*models.AppBackup, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	backupID := fmt.Sprintf("%s-%d", app.Name, time.Now().Unix())
	backupPath := filepath.Join(s.cfg.LogsDir, "backups", app.Name, backupID)

	backup := &models.AppBackup{
		BackupID:  backupID,
		AppID:     app.ID,
		Status:    "in_progress",
		Path:      backupPath,
		SizeBytes: 0,
		Log:       "",
	}
	if err := s.repo.CreateAppBackup(backup); err != nil {
		return nil, fmt.Errorf("failed to create app backup record: %w", err)
	}

	go s.runAppBackup(app, backup, uploadToS3)

	return backup, nil
}

// ListAppBackups lists backups for one app.
func (s *AppService) ListAppBackups(name string, limit int) ([]*models.AppBackup, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	backups, err := s.repo.ListAppBackups(app.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list app backups: %w", err)
	}

	return backups, nil
}

// GetAppBackup gets one backup for an app.
func (s *AppService) GetAppBackup(name, backupID string) (*models.AppBackup, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	backup, err := s.repo.GetAppBackupByBackupID(app.ID, backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app backup: %w", err)
	}
	if backup == nil {
		return nil, fmt.Errorf("backup not found")
	}

	return backup, nil
}

// DeleteAppBackup deletes a backup from storage and database
func (s *AppService) DeleteAppBackup(name string, backupID string) error {
	app, err := s.GetApp(name)
	if err != nil {
		return err
	}

	// Get backup record to find storage path
	backup, err := s.repo.GetAppBackupByBackupID(app.ID, backupID)
	if err != nil {
		return fmt.Errorf("failed to get backup: %w", err)
	}
	if backup == nil {
		return fmt.Errorf("backup not found")
	}

	// Delete from remote storage (S3 or local)
	if backup.S3Path != "" {
		// Backup was stored in S3
		if err := s.backupStorage.Delete(context.Background(), backup.S3Path); err != nil {
			return fmt.Errorf("failed to delete from S3: %w", err)
		}
	} else if backup.Path != "" {
		// Backup is in local storage
		if err := os.RemoveAll(backup.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete local backup: %w", err)
		}
	}

	// Delete from database
	if err := s.repo.DeleteAppBackup(app.ID, backupID); err != nil {
		return fmt.Errorf("failed to delete backup record: %w", err)
	}

	return nil
}

// StartAppRestore starts an asynchronous restore for one app from one backup.
func (s *AppService) StartAppRestore(name, backupID, sourceApp string, restoreAsNew bool, newAppName string, strict bool) (*models.AppRestore, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	targetApp := app
	if restoreAsNew {
		if !isValidAppName(newAppName) {
			return nil, fmt.Errorf("invalid new_app_name: must be lowercase alphanumeric with hyphens, max 32 chars")
		}

		existing, err := s.repo.GetByName(newAppName)
		if err != nil {
			return nil, fmt.Errorf("failed to check target app: %w", err)
		}
		if existing != nil {
			return nil, fmt.Errorf("app with name %s already exists", newAppName)
		}

		envVarsCopy := make(models.EnvVars)
		for k, v := range app.EnvVars {
			envVarsCopy[k] = v
		}

		clone := &models.App{
			Name:           newAppName,
			DeploymentType: app.DeploymentType,
			BuildType:      app.BuildType,
			Status:         "stopped",
			Domains:        []string{},
			EnvVars:        envVarsCopy,
			GitHubRepo:     app.GitHubRepo,
			GitHubBranch:   app.GitHubBranch,
			GitHubSubdir:   app.GitHubSubdir,
			DockerfilePath: app.DockerfilePath,
			ComposePath:    app.ComposePath,
			Port:           app.Port,
		}

		if err := os.MkdirAll(filepath.Join(s.cfg.WorkspacesDir, clone.Name), 0755); err != nil {
			return nil, fmt.Errorf("failed to create new app workspace: %w", err)
		}

		if err := s.repo.Create(clone); err != nil {
			return nil, fmt.Errorf("failed to create new app for restore: %w", err)
		}

		targetApp = clone
	}

	// Try to find the backup in the current app first
	backup, err := s.repo.GetAppBackupByBackupID(app.ID, backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup: %w", err)
	}

	// If not found in current app and sourceApp is provided, look for it in the source app
	if backup == nil && sourceApp != "" {
		sourceAppObj, err := s.GetApp(sourceApp)
		if err == nil {
			backup, _ = s.repo.GetAppBackupByBackupID(sourceAppObj.ID, backupID)
		}
	}

	if backup == nil {
		return nil, fmt.Errorf("backup not found")
	}
	if backup.Status != "success" {
		return nil, fmt.Errorf("backup is not restorable (status: %s)", backup.Status)
	}

	restoreID := fmt.Sprintf("%s-%d", app.Name, time.Now().Unix())
	restore := &models.AppRestore{
		RestoreID: restoreID,
		AppID:     app.ID,
		BackupID:  backupID,
		Status:    "in_progress",
		Log:       "",
	}
	if err := s.repo.CreateAppRestore(restore); err != nil {
		return nil, fmt.Errorf("failed to create app restore record: %w", err)
	}

	go s.runAppRestore(app, targetApp, backup, restore, restoreAsNew, false, strict)

	return restore, nil
}

// StartFreshAppRestore restores a backup from storage into a new app on a fresh machine.
func (s *AppService) StartFreshAppRestore(sourceAppName, backupID, targetAppName string, strict bool) (*models.AppRestore, error) {
	if s.backupStorage == nil {
		return nil, fmt.Errorf("backup storage is required")
	}
	if !isValidAppName(sourceAppName) {
		return nil, fmt.Errorf("invalid source_app: must be lowercase alphanumeric with hyphens, max 32 chars")
	}
	if targetAppName == "" {
		targetAppName = sourceAppName
	}
	if !isValidAppName(targetAppName) {
		return nil, fmt.Errorf("invalid app_name: must be lowercase alphanumeric with hyphens, max 32 chars")
	}

	ctx := context.Background()
	remotePath := s.backupStorage.GetRemotePath(sourceAppName, backupID)
	localPath := filepath.Join(s.cfg.LogsDir, "backups", sourceAppName, backupID)
	if _, err := os.Stat(remotePath); err == nil {
		localPath = remotePath
	} else {
		if err := s.backupStorage.Download(ctx, backupID, remotePath, localPath); err != nil {
			return nil, fmt.Errorf("failed to download backup from storage: %w", err)
		}
	}

	metadata, err := readBackupMetadata(filepath.Join(localPath, "metadata.json"))
	if err != nil {
		return nil, err
	}

	sourceApp := metadata.App
	if sourceApp.Name == "" {
		sourceApp.Name = sourceAppName
	}
	if sourceApp.DeploymentType == "" {
		return nil, fmt.Errorf("backup metadata is missing deployment_type")
	}
	if sourceApp.BuildType == "" {
		return nil, fmt.Errorf("backup metadata is missing build_type")
	}

	if existing, err := s.repo.GetByName(targetAppName); err != nil {
		return nil, fmt.Errorf("failed to check target app: %w", err)
	} else if existing != nil {
		return nil, fmt.Errorf("app with name %s already exists", targetAppName)
	}

	createReq := &models.CreateAppRequest{
		Name:           targetAppName,
		DeploymentType: sourceApp.DeploymentType,
		BuildType:      sourceApp.BuildType,
		Domains:        []string{},
		EnvVars:        sourceApp.EnvVars,
		GitHubRepo:     sourceApp.GitHubRepo,
		GitHubBranch:   sourceApp.GitHubBranch,
		GitHubSubdir:   sourceApp.GitHubSubdir,
		DockerfilePath: sourceApp.DockerfilePath,
		ComposePath:    sourceApp.ComposePath,
		Port:           sourceApp.Port,
	}
	if targetAppName == sourceApp.Name {
		createReq.Domains = sourceApp.Domains
	}
	if sourceApp.DeploymentType == "github" {
		keyPath := filepath.Join(localPath, "deployment.key")
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read deployment key from backup: %w", err)
		}
		createReq.DeploymentKey = string(keyBytes)
	}

	app, err := s.CreateApp(createReq)
	if err != nil {
		return nil, err
	}

	restore := &models.AppRestore{
		RestoreID: fmt.Sprintf("%s-%d", app.Name, time.Now().Unix()),
		AppID:     app.ID,
		BackupID:  backupID,
		Status:    "in_progress",
		Log:       "",
	}
	if err := s.repo.CreateAppRestore(restore); err != nil {
		_ = s.DeleteApp(app.Name)
		return nil, fmt.Errorf("failed to create app restore record: %w", err)
	}

	backup := &models.AppBackup{
		BackupID: backupID,
		Path:     localPath,
		Status:   "success",
	}
	go s.runAppRestore(&sourceApp, app, backup, restore, targetAppName != sourceApp.Name, true, strict)

	return restore, nil
}

// GetAppRestore gets one restore run for an app.
func (s *AppService) GetAppRestore(name, restoreID string) (*models.AppRestore, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	restore, err := s.repo.GetAppRestoreByRestoreID(app.ID, restoreID)
	if err != nil {
		return nil, fmt.Errorf("failed to get app restore: %w", err)
	}
	if restore == nil {
		restore, err = s.repo.GetLatestAppRestoreByBackupID(app.ID, restoreID)
		if err != nil {
			return nil, fmt.Errorf("failed to get app restore by backup id: %w", err)
		}
	}
	if restore == nil {
		return nil, fmt.Errorf("restore not found")
	}

	return restore, nil
}

func (s *AppService) runAppBackup(app *models.App, backup *models.AppBackup, uploadToS3 bool) {
	var logBuilder strings.Builder
	ctx := context.Background()

	appendLog := func(line string) {
		logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), line))
	}

	appendLog(fmt.Sprintf("Starting backup for app %s", app.Name))
	appendLog("Waiting for global backup lock")
	s.backupMu.Lock()
	appendLog("Acquired global backup lock")
	defer s.backupMu.Unlock()

	frozenContainers, err := s.freezeAllManagedContainers(ctx, appendLog)
	if err != nil {
		s.finishBackupWithError(backup, &logBuilder, err)
		return
	}

	sizeBytes, runErr := s.runAppBackupPayload(ctx, app, backup, uploadToS3, appendLog)
	restartErr := s.restartFrozenContainers(ctx, frozenContainers, appendLog)

	if restartErr != nil {
		if runErr != nil {
			runErr = fmt.Errorf("%w; additionally failed to restart frozen containers: %v", runErr, restartErr)
		} else {
			runErr = fmt.Errorf("backup artifacts created, but failed to restart frozen containers: %w", restartErr)
		}
	}

	if runErr != nil {
		s.finishBackupWithError(backup, &logBuilder, runErr)
		return
	}

	now := time.Now()
	backup.Status = "success"
	backup.SizeBytes = sizeBytes
	backup.Log = logBuilder.String()
	backup.CompletedAt = &now
	_ = s.repo.UpdateAppBackup(backup)
}

func (s *AppService) runAppBackupPayload(ctx context.Context, app *models.App, backup *models.AppBackup, uploadToS3 bool, appendLog func(string)) (int64, error) {
	if err := os.MkdirAll(filepath.Join(backup.Path, "volumes"), 0755); err != nil {
		return 0, fmt.Errorf("failed to create backup directory: %w", err)
	}

	deploymentLogs, err := s.repo.GetDeploymentLogs(app.ID, 100)
	if err != nil {
		return 0, fmt.Errorf("failed to load deployment logs: %w", err)
	}

	metadata := map[string]interface{}{
		"backup_id":       backup.BackupID,
		"app":             app,
		"deployment_logs": deploymentLogs,
		"created_at":      time.Now().UTC(),
	}

	metadataPath := filepath.Join(backup.Path, "metadata.json")
	if err := writeJSONFile(metadataPath, metadata); err != nil {
		return 0, fmt.Errorf("failed to write metadata: %w", err)
	}
	appendLog("Saved metadata.json")

	workspaceDir := filepath.Join(s.cfg.WorkspacesDir, app.Name)
	if _, err := os.Stat(workspaceDir); err == nil {
		workspaceArchive := filepath.Join(backup.Path, "workspace.tar.gz")
		if err := tarGzDirectory(workspaceDir, workspaceArchive); err != nil {
			return 0, fmt.Errorf("failed to archive workspace: %w", err)
		}
		appendLog("Archived workspace")
	}

	keyPath := filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", app.Name))
	if _, err := os.Stat(keyPath); err == nil {
		backupKeyPath := filepath.Join(backup.Path, "deployment.key")
		if err := copyFile(keyPath, backupKeyPath, 0600); err != nil {
			return 0, fmt.Errorf("failed to copy deployment key: %w", err)
		}
		appendLog("Copied deployment key")
	}

	volumeNames, err := s.deploymentService.dockerService.GetVolumeNamesByApp(ctx, app.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to discover app volumes: %w", err)
	}

	for _, volumeName := range volumeNames {
		fileName := sanitizeVolumeArchiveName(volumeName)
		targetFile := filepath.Join(backup.Path, "volumes", fileName)
		if err := backupDockerVolume(ctx, volumeName, targetFile); err != nil {
			return 0, fmt.Errorf("failed to backup volume %s: %w", volumeName, err)
		}
		appendLog(fmt.Sprintf("Backed up volume %s", volumeName))
	}

	sizeBytes, err := directorySize(backup.Path)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate backup size: %w", err)
	}

	// Upload to S3 if requested
	if uploadToS3 && s.backupStorage != nil {
		appendLog("Uploading backup to S3...")
		s3Path, err := s.backupStorage.Upload(ctx, backup.Path, backup.BackupID, app.Name)
		if err != nil {
			// Log the error but don't fail the backup - S3 is optional
			appendLog(fmt.Sprintf("Warning: Failed to upload to S3: %v", err))
		} else {
			backup.S3Path = s3Path
			backup.UploadedToS3 = true
			uploadTime := time.Now()
			backup.S3UploadedAt = &uploadTime
			appendLog(fmt.Sprintf("Successfully uploaded to S3: %s", s3Path))
		}
	}

	return sizeBytes, nil
}

type frozenContainer struct {
	ID   string
	Name string
}

func (s *AppService) freezeAllManagedContainers(ctx context.Context, appendLog func(string)) ([]frozenContainer, error) {
	apps, err := s.repo.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list apps for backup freeze: %w", err)
	}

	containerByID := make(map[string]frozenContainer)
	for _, app := range apps {
		if app == nil || app.Name == "" || s.isDestroyedApp(app) {
			continue
		}

		containersByAppLabel, err := s.deploymentService.dockerService.GetContainersByLabel(ctx, "atmosphere.app", app.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to list containers by app label for %s: %w", app.Name, err)
		}

		projectName := fmt.Sprintf("atmosphere-%s", app.Name)
		containersByComposeProject, err := s.deploymentService.dockerService.GetContainersByComposeProject(ctx, projectName)
		if err != nil {
			return nil, fmt.Errorf("failed to list compose project containers for %s: %w", app.Name, err)
		}

		containersByNamePrefix, err := s.deploymentService.dockerService.GetContainersByNamePrefix(ctx, app.Name+"-")
		if err != nil {
			return nil, fmt.Errorf("failed to list containers by name prefix for %s: %w", app.Name, err)
		}

		groups := [][]types.Container{containersByAppLabel, containersByComposeProject, containersByNamePrefix}
		for _, group := range groups {
			for _, c := range group {
				if c.State != "running" {
					continue
				}
				if _, exists := containerByID[c.ID]; exists {
					continue
				}
				name := c.ID
				if len(name) > 12 {
					name = name[:12]
				}
				if len(c.Names) > 0 && c.Names[0] != "" {
					name = strings.TrimPrefix(c.Names[0], "/")
				}
				containerByID[c.ID] = frozenContainer{ID: c.ID, Name: name}
			}
		}
	}

	running := make([]frozenContainer, 0, len(containerByID))
	for _, container := range containerByID {
		running = append(running, container)
	}

	sort.Slice(running, func(i, j int) bool {
		if running[i].Name == running[j].Name {
			return running[i].ID < running[j].ID
		}
		return running[i].Name < running[j].Name
	})

	if len(running) == 0 {
		appendLog("No running managed containers to freeze")
		return nil, nil
	}

	appendLog(fmt.Sprintf("Freezing %d running managed containers", len(running)))
	frozen := make([]frozenContainer, 0, len(running))
	for _, container := range running {
		if err := s.deploymentService.dockerService.StopContainer(ctx, container.ID); err != nil {
			_ = s.restartFrozenContainers(ctx, frozen, appendLog)
			return nil, fmt.Errorf("failed to stop container %s: %w", container.Name, err)
		}
		frozen = append(frozen, container)
		appendLog(fmt.Sprintf("Stopped %s", container.Name))
	}

	return frozen, nil
}

func (s *AppService) restartFrozenContainers(ctx context.Context, frozen []frozenContainer, appendLog func(string)) error {
	if len(frozen) == 0 {
		return nil
	}

	appendLog(fmt.Sprintf("Restarting %d frozen containers", len(frozen)))
	var restartErrs []string
	for _, container := range frozen {
		if err := s.deploymentService.dockerService.StartContainer(ctx, container.ID); err != nil {
			restartErrs = append(restartErrs, fmt.Sprintf("%s: %v", container.Name, err))
			appendLog(fmt.Sprintf("Failed to restart %s: %v", container.Name, err))
			continue
		}
		appendLog(fmt.Sprintf("Restarted %s", container.Name))
	}

	if len(restartErrs) > 0 {
		return fmt.Errorf("one or more containers failed to restart: %s", strings.Join(restartErrs, "; "))
	}

	return nil
}

func (s *AppService) runAppRestore(sourceApp *models.App, targetApp *models.App, backup *models.AppBackup, restore *models.AppRestore, restoreAsNew bool, deployFromSnapshot bool, strict bool) {
	var logBuilder strings.Builder
	ctx := context.Background()

	appendLog := func(line string) {
		logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), line))
	}

	if strict {
		appendLog("Restore strict mode: enabled")
	} else {
		appendLog("Restore strict mode: disabled (best-effort)")
	}

	appendLog(fmt.Sprintf("Starting restore for app %s from %s", targetApp.Name, backup.BackupID))

	backupPath := backup.Path

	// If backup doesn't exist locally but is in S3, download it
	if _, err := os.Stat(backupPath); err != nil && os.IsNotExist(err) {
		if backup.UploadedToS3 && backup.S3Path != "" && s.backupStorage != nil {
			appendLog(fmt.Sprintf("Backup not found locally, downloading from S3: %s", backup.S3Path))
			// Create local backup directory for download
			if err := os.MkdirAll(backupPath, 0755); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to create backup directory: %w", err))
				return
			}

			if err := s.backupStorage.Download(ctx, backup.BackupID, backup.S3Path, backupPath); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to download from S3: %w", err))
				return
			}
			appendLog(fmt.Sprintf("Successfully downloaded from S3 to %s", backupPath))
		} else {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("backup path unavailable: %w", err))
			return
		}
	}

	wasRunning := !deployFromSnapshot && !restoreAsNew && sourceApp.Status == "running"
	if wasRunning {
		if err := s.deploymentService.Stop(ctx, sourceApp); err != nil {
			appendLog(fmt.Sprintf("Warning: failed to stop running app before restore: %v", err))
		} else {
			appendLog("Stopped app containers before restore")
		}
	}

	workspaceArchive := filepath.Join(backupPath, "workspace.tar.gz")
	if _, err := os.Stat(workspaceArchive); err == nil {
		workspaceDir := filepath.Join(s.cfg.WorkspacesDir, targetApp.Name)
		if err := os.RemoveAll(workspaceDir); err != nil {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to clean workspace: %w", err))
			return
		}
		if err := os.MkdirAll(workspaceDir, 0755); err != nil {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to create workspace: %w", err))
			return
		}
		if err := extractTarGz(workspaceArchive, workspaceDir); err != nil {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to restore workspace: %w", err))
			return
		}
		if uid, gid, ok := getEffectiveOwnership(); ok {
			_ = recursiveChown(workspaceDir, uid, gid)
			appendLog(fmt.Sprintf("Restored workspace and normalized ownership to uid=%d gid=%d", uid, gid))
		} else {
			appendLog("Restored workspace")
		}
	}

	backupKeyPath := filepath.Join(backupPath, "deployment.key")
	if _, err := os.Stat(backupKeyPath); err == nil {
		targetKeyPath := filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", targetApp.Name))
		if err := os.MkdirAll(s.cfg.KeysDir, 0700); err != nil {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to prepare keys directory: %w", err))
			return
		}
		if err := copyFile(backupKeyPath, targetKeyPath, 0600); err != nil {
			s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to restore deployment key: %w", err))
			return
		}
		appendLog("Restored deployment key")
	}

	volumesDir := filepath.Join(backupPath, "volumes")
	entries, err := os.ReadDir(volumesDir)
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

		volumeRestorePlan, err := validateVolumeRestorePlan(entries, sourceApp.Name, targetApp.Name, restoreAsNew, strict)
		if err != nil {
			s.finishRestoreWithError(restore, &logBuilder, err)
			return
		}
		for _, warning := range volumeRestorePlan.Warnings {
			appendLog("Volume mapping warning: " + warning)
		}

		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
				continue
			}
			if volumeRestorePlan.ShouldSkipArchive(entry.Name()) {
				appendLog(fmt.Sprintf("Skipping volume archive %s due to mapping collision in best-effort mode", entry.Name()))
				continue
			}
			sourceVolumeName := restoreVolumeNameFromArchive(entry.Name())
			if sourceVolumeName == "" {
				appendLog(fmt.Sprintf("Skipping invalid volume archive name: %s", entry.Name()))
				continue
			}

			targetVolumeName := mapVolumeNameForRestore(sourceVolumeName, sourceApp.Name, targetApp.Name, restoreAsNew)

			if err := s.deploymentService.dockerService.EnsureVolume(ctx, targetVolumeName); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to ensure volume %s: %w", targetVolumeName, err))
				return
			}

			sourceFile := filepath.Join(volumesDir, entry.Name())
			if err := restoreDockerVolume(ctx, targetVolumeName, sourceFile); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to restore volume %s: %w", targetVolumeName, err))
				return
			}
			// Do NOT recursiveChown volume data. The tar archive preserves the exact
			// UID/GID of files as they were inside the container (e.g. postgres=999,
			// openproject=1000). Forcing Atmosphere's process UID on volume data
			// breaks container databases that refuse to start if their data directory
			// is not owned by the expected internal user.
			appendLog(fmt.Sprintf("Restored volume %s (source %s)", targetVolumeName, sourceVolumeName))
		}
	}

	if deployFromSnapshot {
		if err := s.validateRestoreWorkspace(targetApp); err != nil {
			if strict {
				s.finishRestoreWithError(restore, &logBuilder, err)
				return
			}
			appendLog(fmt.Sprintf("Warning: restore preflight check failed in best-effort mode: %v", err))
		}

		appendLog("Deploying restored app from workspace snapshot (no Git sync)")
		deployLog, err := s.deploymentService.DeployFromWorkspace(ctx, targetApp)
		logBuilder.WriteString(deployLog)
		if err != nil {
			s.finishRestoreWithError(restore, &logBuilder, err)
			return
		}

		appendLog("Validating restored containers runtime state")
		if err := s.verifyRestoreRuntimeHealth(ctx, targetApp, appendLog); err != nil {
			if strict {
				s.finishRestoreWithError(restore, &logBuilder, err)
				return
			}
			appendLog(fmt.Sprintf("Warning: post-restore runtime validation failed in best-effort mode: %v", err))
		}

		now := time.Now()
		targetApp.Status = "running"
		targetApp.LastDeployedAt = &now
		if err := s.repo.Update(targetApp); err != nil {
			appendLog(fmt.Sprintf("Warning: failed to persist app status after restored deployment: %v", err))
		}
	} else if wasRunning {
		if err := s.deploymentService.Restart(ctx, sourceApp); err != nil {
			appendLog(fmt.Sprintf("Warning: failed to restart app after restore: %v", err))
		} else {
			appendLog("Restarted app containers")
		}
	}

	now := time.Now()
	restore.Status = "success"
	restore.Log = logBuilder.String()
	restore.CompletedAt = &now
	_ = s.repo.UpdateAppRestore(restore)
}

func (s *AppService) finishBackupWithError(backup *models.AppBackup, logBuilder *strings.Builder, err error) {
	logBuilder.WriteString(fmt.Sprintf("[%s] Error: %v\n", time.Now().Format("15:04:05"), err))
	now := time.Now()
	backup.Status = "failed"
	backup.Log = logBuilder.String()
	backup.CompletedAt = &now
	_ = s.repo.UpdateAppBackup(backup)
}

func (s *AppService) finishRestoreWithError(restore *models.AppRestore, logBuilder *strings.Builder, err error) {
	logBuilder.WriteString(fmt.Sprintf("[%s] Error: %v\n", time.Now().Format("15:04:05"), err))
	now := time.Now()
	restore.Status = "failed"
	restore.Log = logBuilder.String()
	restore.CompletedAt = &now
	_ = s.repo.UpdateAppRestore(restore)
}

// recursiveChown sets ownership for all files/dirs under root to the given uid/gid.
func recursiveChown(root string, uid, gid int) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err == nil {
			_ = os.Chown(path, uid, gid)
		}
		return nil
	})
}

func getEffectiveOwnership() (int, int, bool) {
	uid := os.Geteuid()
	gid := os.Getegid()
	if uid < 0 || gid < 0 {
		return 0, 0, false
	}
	return uid, gid, true
}

type backupMetadata struct {
	App models.App `json:"app"`
}

func readBackupMetadata(metadataPath string) (*backupMetadata, error) {
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup metadata: %w", err)
	}

	var metadata backupMetadata
	if err := json.Unmarshal(content, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse backup metadata: %w", err)
	}

	return &metadata, nil
}

func writeJSONFile(path string, data interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Chmod(mode)
}

func tarGzDirectory(srcDir, targetFile string) error {
	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tw, file)
		return err
	})
}

func extractTarGz(archivePath, targetDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		targetPath := filepath.Join(targetDir, filepath.Clean(header.Name))
		if !isPathWithin(targetPath, targetDir) {
			return fmt.Errorf("archive entry outside target directory: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}

			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			if err := outFile.Close(); err != nil {
				return err
			}
		}
	}

	return nil
}

func directorySize(root string) (int64, error) {
	var size int64
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return size, nil
}

func sanitizeVolumeArchiveName(volumeName string) string {
	encoded := base64.RawURLEncoding.EncodeToString([]byte(volumeName))
	return encoded + ".tar.gz"
}

func restoreVolumeNameFromArchive(archiveName string) string {
	if !strings.HasSuffix(archiveName, ".tar.gz") {
		return ""
	}
	encoded := strings.TrimSuffix(archiveName, ".tar.gz")
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return string(decoded)
}

func backupDockerVolume(ctx context.Context, volumeName, targetFile string) error {
	backupDir := filepath.Dir(targetFile)
	archiveName := filepath.Base(targetFile)

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	cmd := exec.CommandContext(
		ctx,
		"docker",
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:/source:ro", volumeName),
		"-v", fmt.Sprintf("%s:/backup", backupDir),
		"busybox",
		"sh",
		"-c",
		fmt.Sprintf("cd /source && tar -czf /backup/%s .", archiveName),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker volume backup command failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func restoreDockerVolume(ctx context.Context, volumeName, sourceFile string) error {
	backupDir := filepath.Dir(sourceFile)
	archiveName := filepath.Base(sourceFile)

	cmd := exec.CommandContext(
		ctx,
		"docker",
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:/target", volumeName),
		"-v", fmt.Sprintf("%s:/backup", backupDir),
		"busybox",
		"sh",
		"-c",
		fmt.Sprintf("mkdir -p /target && rm -rf /target/* /target/.[!.]* /target/..?* 2>/dev/null || true; tar -xzf /backup/%s -C /target", archiveName),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker volume restore command failed: %w (%s)", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func mapVolumeNameForRestore(sourceVolumeName, sourceAppName, targetAppName string, restoreAsNew bool) string {
	if !restoreAsNew || sourceAppName == targetAppName {
		return sourceVolumeName
	}

	// Compose project volumes usually look like atmosphere-<app>_<volume>.
	prefix := fmt.Sprintf("atmosphere-%s_", sourceAppName)
	if strings.HasPrefix(sourceVolumeName, prefix) {
		suffix := strings.TrimPrefix(sourceVolumeName, prefix)
		return fmt.Sprintf("atmosphere-%s_%s", targetAppName, suffix)
	}

	// Fallback replacement for custom names containing source app name.
	return strings.ReplaceAll(sourceVolumeName, sourceAppName, targetAppName)
}

type volumeRestorePlan struct {
	Warnings       []string
	skippedArchive map[string]struct{}
}

func (p *volumeRestorePlan) ShouldSkipArchive(archiveName string) bool {
	if p == nil || len(p.skippedArchive) == 0 {
		return false
	}
	_, exists := p.skippedArchive[archiveName]
	return exists
}

func validateVolumeRestorePlan(entries []os.DirEntry, sourceAppName, targetAppName string, restoreAsNew bool, strict bool) (*volumeRestorePlan, error) {
	plan := &volumeRestorePlan{}
	targetToSource := make(map[string]string)
	plan.skippedArchive = make(map[string]struct{})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
			continue
		}

		sourceVolumeName := restoreVolumeNameFromArchive(entry.Name())
		if sourceVolumeName == "" {
			continue
		}

		targetVolumeName := mapVolumeNameForRestore(sourceVolumeName, sourceAppName, targetAppName, restoreAsNew)
		if existingSource, exists := targetToSource[targetVolumeName]; exists && existingSource != sourceVolumeName {
			if !strict {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("ambiguous volume mapping: %s and %s both map to %s; keeping first and skipping archive %s", existingSource, sourceVolumeName, targetVolumeName, entry.Name()))
				plan.skippedArchive[entry.Name()] = struct{}{}
				continue
			}
			return nil, fmt.Errorf(
				"ambiguous volume mapping: source volumes %s and %s both map to target volume %s",
				existingSource,
				sourceVolumeName,
				targetVolumeName,
			)
		}
		targetToSource[targetVolumeName] = sourceVolumeName

		if restoreAsNew && sourceAppName != targetAppName && targetVolumeName == sourceVolumeName {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("%s kept original name (external/custom volume naming may collide on shared host)", sourceVolumeName))
		}
	}

	sort.Strings(plan.Warnings)
	return plan, nil
}

func (s *AppService) validateRestoreWorkspace(app *models.App) error {
	workspaceDir := filepath.Join(s.cfg.WorkspacesDir, app.Name)
	if _, err := os.Stat(workspaceDir); err != nil {
		return fmt.Errorf("restored workspace missing at %s: %w", workspaceDir, err)
	}

	buildDir := workspaceDir
	if app.GitHubSubdir != "" {
		buildDir = filepath.Join(workspaceDir, app.GitHubSubdir)
	}

	if _, err := os.Stat(buildDir); err != nil {
		return fmt.Errorf("restored build directory missing at %s: %w", buildDir, err)
	}

	if app.BuildType == "compose" {
		var composePath string
		if app.ComposePath != "" {
			composePath = filepath.Join(buildDir, app.ComposePath)
		} else {
			composePath = s.deploymentService.DetectComposeFile(buildDir)
		}
		if composePath == "" {
			return fmt.Errorf("restore preflight failed: no compose file found in %s", buildDir)
		}
		if _, err := os.Stat(composePath); err != nil {
			return fmt.Errorf("restore preflight failed: compose file missing at %s: %w", composePath, err)
		}
		return nil
	}

	dockerfilePath := filepath.Join(buildDir, "Dockerfile")
	if app.DockerfilePath != "" {
		dockerfilePath = filepath.Join(buildDir, app.DockerfilePath)
	}
	if _, err := os.Stat(dockerfilePath); err != nil {
		return fmt.Errorf("restore preflight failed: Dockerfile missing at %s: %w", dockerfilePath, err)
	}

	return nil
}

func (s *AppService) verifyRestoreRuntimeHealth(ctx context.Context, app *models.App, appendLog func(string)) error {
	const maxAttempts = 6
	const attemptDelay = 2 * time.Second

	var lastIssues []string
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		issues, err := s.deploymentService.dockerService.GetAppRuntimeIssues(ctx, app.Name)
		if err != nil {
			return fmt.Errorf("post-restore runtime check failed: %w", err)
		}
		if len(issues) == 0 {
			appendLog("Container runtime validation passed")
			return nil
		}

		lastIssues = issues
		appendLog(fmt.Sprintf("Runtime validation attempt %d/%d found issues: %s", attempt, maxAttempts, strings.Join(issues, "; ")))
		if attempt < maxAttempts {
			time.Sleep(attemptDelay)
		}
	}

	return fmt.Errorf("post-restore runtime validation failed: %s", strings.Join(lastIssues, "; "))
}
