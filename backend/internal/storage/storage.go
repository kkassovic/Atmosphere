package storage

import (
	"context"
	"io"
)

// BackupStorage is an interface for storing and retrieving backups
type BackupStorage interface {
	// Upload uploads a local backup directory to remote storage
	// Returns the remote path/identifier
	Upload(ctx context.Context, localPath string, backupID string, appName string) (remotePath string, err error)

	// Download downloads a backup from remote storage to a local path
	// Creates the directory if it doesn't exist
	Download(ctx context.Context, backupID string, remotePath string, localPath string) error

	// Delete removes a backup from remote storage
	Delete(ctx context.Context, remotePath string) error

	// Exists checks if a backup exists in remote storage
	Exists(ctx context.Context, remotePath string) (bool, error)

	// List lists all backups for an app in remote storage
	List(ctx context.Context, appName string) ([]string, error)

	// GetRemotePath returns the remote path for a backup
	// Used for storing/tracking in database
	GetRemotePath(appName string, backupID string) string
}

// UploadResult contains information about an uploaded backup
type UploadResult struct {
	RemotePath string
	SizeBytes  int64
}

// DownloadProgress tracks download progress
type DownloadProgress struct {
	TotalBytes     int64
	DownloadedBytes int64
}

// ProgressCallback is called periodically during upload/download
type ProgressCallback func(progress DownloadProgress)
