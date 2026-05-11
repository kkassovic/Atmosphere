package storage

import (
	"context"
	"fmt"
	"os"
)

// LocalStorage implements BackupStorage for local filesystem
type LocalStorage struct {
	baseDir string
}

// NewLocalStorage creates a new local storage handler
func NewLocalStorage(baseDir string) (*LocalStorage, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory is required")
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &LocalStorage{
		baseDir: baseDir,
	}, nil
}

// Upload is a no-op for local storage (backup already on disk)
func (l *LocalStorage) Upload(ctx context.Context, localPath string, backupID string, appName string) (string, error) {
	// Local storage doesn't need to upload anywhere
	// The remotePath is just the local path itself
	return l.GetRemotePath(appName, backupID), nil
}

// Download is a no-op for local storage (file already on disk)
func (l *LocalStorage) Download(ctx context.Context, backupID string, remotePath string, localPath string) error {
	// For local storage, remotePath should already be the local path
	// This is a no-op since the backup is already accessible locally
	if remotePath != localPath {
		return fmt.Errorf("local storage: remotePath and localPath must be the same")
	}
	return nil
}

// Delete removes a backup from local storage
func (l *LocalStorage) Delete(ctx context.Context, remotePath string) error {
	if remotePath == "" {
		return fmt.Errorf("remotePath cannot be empty")
	}

	if err := os.RemoveAll(remotePath); err != nil {
		return fmt.Errorf("failed to delete local backup: %w", err)
	}

	return nil
}

// Exists checks if a backup exists in local storage
func (l *LocalStorage) Exists(ctx context.Context, remotePath string) (bool, error) {
	if remotePath == "" {
		return false, nil
	}

	_, err := os.Stat(remotePath)
	if err == nil {
		return true, nil
	}

	if os.IsNotExist(err) {
		return false, nil
	}

	return false, err
}

// List lists all backups for an app in local storage
func (l *LocalStorage) List(ctx context.Context, appName string) ([]string, error) {
	appDir := l.GetRemotePath(appName, "")

	entries, err := os.ReadDir(appDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil // App has no backups yet
		}
		return nil, fmt.Errorf("failed to list local backups: %w", err)
	}

	var backups []string
	for _, entry := range entries {
		if entry.IsDir() {
			backups = append(backups, entry.Name())
		}
	}

	return backups, nil
}

// GetRemotePath returns the local path for a backup
func (l *LocalStorage) GetRemotePath(appName string, backupID string) string {
	if backupID == "" {
		return fmt.Sprintf("%s/backups/%s", l.baseDir, appName)
	}
	return fmt.Sprintf("%s/backups/%s/%s", l.baseDir, appName, backupID)
}
