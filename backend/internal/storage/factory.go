package storage

import (
	"fmt"
)

// StorageConfig holds configuration for backup storage
type StorageConfig struct {
	// Type can be "local" or "s3"
	Type string

	// For local storage
	LocalBasePath string

	// For S3 storage
	S3Endpoint    string
	S3Bucket      string
	S3Region      string
	S3AccessKey   string
	S3SecretKey   string
	S3PathPrefix  string
}

// NewBackupStorage creates and returns the configured backup storage backend
func NewBackupStorage(cfg *StorageConfig) (BackupStorage, error) {
	if cfg == nil {
		return nil, fmt.Errorf("storage config is required")
	}

	switch cfg.Type {
	case "s3":
		return NewS3Storage(
			cfg.S3Endpoint,
			cfg.S3Bucket,
			cfg.S3Region,
			cfg.S3AccessKey,
			cfg.S3SecretKey,
			cfg.S3PathPrefix,
		)
	case "local", "":
		// Default to local storage
		return NewLocalStorage(cfg.LocalBasePath)
	default:
		return nil, fmt.Errorf("unknown storage type: %s", cfg.Type)
	}
}
