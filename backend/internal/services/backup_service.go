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
func (s *AppService) CreateAppBackup(name string) (*models.AppBackup, error) {
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

	go s.runAppBackup(app, backup)

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

// StartAppRestore starts an asynchronous restore for one app from one backup.
func (s *AppService) StartAppRestore(name, backupID string) (*models.AppRestore, error) {
	app, err := s.GetApp(name)
	if err != nil {
		return nil, err
	}

	backup, err := s.repo.GetAppBackupByBackupID(app.ID, backupID)
	if err != nil {
		return nil, fmt.Errorf("failed to load backup: %w", err)
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

	go s.runAppRestore(app, backup, restore)

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
		return nil, fmt.Errorf("restore not found")
	}

	return restore, nil
}

func (s *AppService) runAppBackup(app *models.App, backup *models.AppBackup) {
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

	now := time.Now()
	backup.Status = "success"
	backup.SizeBytes = sizeBytes
	backup.Log = logBuilder.String()
	backup.CompletedAt = &now
	_ = s.repo.UpdateAppBackup(backup)
}

func (s *AppService) runAppRestore(app *models.App, backup *models.AppBackup, restore *models.AppRestore) {
	var logBuilder strings.Builder
	ctx := context.Background()

	appendLog := func(line string) {
		logBuilder.WriteString(fmt.Sprintf("[%s] %s\n", time.Now().Format("15:04:05"), line))
	}

	appendLog(fmt.Sprintf("Starting restore for app %s from %s", app.Name, backup.BackupID))

	if _, err := os.Stat(backup.Path); err != nil {
		s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("backup path unavailable: %w", err))
		return
	}

	wasRunning := app.Status == "running"
	if wasRunning {
		if err := s.deploymentService.Stop(ctx, app); err != nil {
			appendLog(fmt.Sprintf("Warning: failed to stop running app before restore: %v", err))
		} else {
			appendLog("Stopped app containers before restore")
		}
	}

	workspaceArchive := filepath.Join(backup.Path, "workspace.tar.gz")
	if _, err := os.Stat(workspaceArchive); err == nil {
		workspaceDir := filepath.Join(s.cfg.WorkspacesDir, app.Name)
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
		appendLog("Restored workspace")
	}

	backupKeyPath := filepath.Join(backup.Path, "deployment.key")
	if _, err := os.Stat(backupKeyPath); err == nil {
		targetKeyPath := filepath.Join(s.cfg.KeysDir, fmt.Sprintf("%s.key", app.Name))
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

	volumesDir := filepath.Join(backup.Path, "volumes")
	entries, err := os.ReadDir(volumesDir)
	if err == nil {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tar.gz") {
				continue
			}
			volumeName := restoreVolumeNameFromArchive(entry.Name())
			if volumeName == "" {
				appendLog(fmt.Sprintf("Skipping invalid volume archive name: %s", entry.Name()))
				continue
			}

			if err := s.deploymentService.dockerService.EnsureVolume(ctx, volumeName); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to ensure volume %s: %w", volumeName, err))
				return
			}

			sourceFile := filepath.Join(volumesDir, entry.Name())
			if err := restoreDockerVolume(ctx, volumeName, sourceFile); err != nil {
				s.finishRestoreWithError(restore, &logBuilder, fmt.Errorf("failed to restore volume %s: %w", volumeName, err))
				return
			}
			appendLog(fmt.Sprintf("Restored volume %s", volumeName))
		}
	}

	if wasRunning {
		if err := s.deploymentService.Start(ctx, app); err != nil {
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
