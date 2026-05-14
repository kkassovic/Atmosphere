package services

import (
	"archive/tar"
	"atmosphere/internal/models"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
func (s *AppService) StartAppRestore(name, backupID, sourceApp string, restoreAsNew bool, newAppName string) (*models.AppRestore, error) {
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

	go s.runAppRestore(app, targetApp, backup, restore, restoreAsNew)

	return restore, nil
}

// StartFreshAppRestore restores a backup from storage into a new app on a fresh machine.
func (s *AppService) StartFreshAppRestore(sourceAppName, backupID, targetAppName string) (*models.AppRestore, error) {
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
	go s.runAppRestore(&sourceApp, app, backup, restore, targetAppName != sourceApp.Name)

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

	if err := os.MkdirAll(filepath.Join(backup.Path, "volumes"), 0755); err != nil {
		s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to create backup directory: %w", err))
		return
	}

	deploymentLogs, err := s.repo.GetDeploymentLogs(app.ID, 100)
	if err != nil {
		s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to load deployment logs: %w", err))
		return
	}

	metadata := map[string]interface{}{
		"backup_id":       backup.BackupID,
		"app":             app,
		"deployment_logs": deploymentLogs,
		"created_at":      time.Now().UTC(),
	}

	metadataPath := filepath.Join(backup.Path, "metadata.json")
	if err := writeJSONFile(metadataPath, metadata); err != nil {
		s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to write metadata: %w", err))
		return
	}
	appendLog("Saved metadata.json")

	workspaceDir := filepath.Join(s.cfg.WorkspacesDir, app.Name)
	if _, err := os.Stat(workspaceDir); err == nil {
		workspaceArchive := filepath.Join(backup.Path, "workspace.tar.gz")
		if err := tarGzDirectory(workspaceDir, workspaceArchive); err != nil {
			s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to archive workspace: %w", err))
			return
		}
		appendLog("Archived workspace")
	}

	keyPath := filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", app.Name))
	if _, err := os.Stat(keyPath); err == nil {
		backupKeyPath := filepath.Join(backup.Path, "deployment.key")
		if err := copyFile(keyPath, backupKeyPath, 0600); err != nil {
			s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to copy deployment key: %w", err))
			return
		}
		appendLog("Copied deployment key")
	}

	volumeNames, err := s.deploymentService.dockerService.GetVolumeNamesByApp(ctx, app.Name)
	if err != nil {
		s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to discover app volumes: %w", err))
		return
	}

	for _, volumeName := range volumeNames {
		fileName := sanitizeVolumeArchiveName(volumeName)
		targetFile := filepath.Join(backup.Path, "volumes", fileName)
		if err := backupDockerVolume(ctx, volumeName, targetFile); err != nil {
			s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to backup volume %s: %w", volumeName, err))
			return
		}
		appendLog(fmt.Sprintf("Backed up volume %s", volumeName))
	}

	sizeBytes, err := directorySize(backup.Path)
	if err != nil {
		s.finishBackupWithError(backup, &logBuilder, fmt.Errorf("failed to calculate backup size: %w", err))
		return
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

	now := time.Now()
	backup.Status = "success"
	backup.SizeBytes = sizeBytes
	backup.Log = logBuilder.String()
	backup.CompletedAt = &now
	_ = s.repo.UpdateAppBackup(backup)
}

func (s *AppService) runAppRestore(sourceApp *models.App, targetApp *models.App, backup *models.AppBackup, restore *models.AppRestore, restoreAsNew bool) {
	var logBuilder strings.Builder
	ctx := context.Background()

	appendLog := func(line string) {
		logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), line))
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

	wasRunning := !restoreAsNew && sourceApp.Status == "running"
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
		   // Ensure workspace files are owned by UID/GID 1000 (OpenProject user)
		   _ = recursiveChown(workspaceDir, 1000, 1000)
		   appendLog("Restored workspace and fixed ownership")
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
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
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
			// After restoring the volume, fix ownership inside the volume mount.
			if mountPath, err := s.deploymentService.dockerService.GetVolumeMountpoint(ctx, targetVolumeName); err == nil {
				_ = recursiveChown(mountPath, 1000, 1000)
			}
			appendLog(fmt.Sprintf("Restored volume %s (source %s) and fixed ownership", targetVolumeName, sourceVolumeName))
		}
	}

	if wasRunning {
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
		fmt.Sprintf("mkdir -p /target && tar -xzf /backup/%s -C /target", archiveName),
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
