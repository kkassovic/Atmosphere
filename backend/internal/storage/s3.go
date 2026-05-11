package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Storage implements BackupStorage for S3-compatible services (DigitalOcean Spaces, AWS S3, etc)
type S3Storage struct {
	client      *s3.Client
	bucket      string
	pathPrefix  string
	endpoint    string
}

// NewS3Storage creates a new S3 storage handler
func NewS3Storage(endpoint, bucket, region, accessKey, secretKey, pathPrefix string) (*S3Storage, error) {
	if endpoint == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("S3 endpoint, bucket, accessKey, and secretKey are required")
	}

	pathPrefix = strings.TrimSpace(pathPrefix)
	pathPrefix = strings.ReplaceAll(pathPrefix, "\\", "/")
	pathPrefix = strings.TrimLeft(pathPrefix, "-/")
	pathPrefix = strings.TrimRight(pathPrefix, "/")

	// Create custom resolver for S3-compatible services
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if service == s3.ServiceID {
			return aws.Endpoint{
				URL:           endpoint,
				SigningRegion: region,
			}, nil
		}
		return aws.Endpoint{}, fmt.Errorf("unknown service")
	})

	cfg := aws.NewConfig()
	cfg.EndpointResolverWithOptions = resolver
	cfg.Credentials = credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")
	cfg.Region = region
	if region == "" {
		cfg.Region = "us-east-1" // Default region for S3-compatible services
	}

	client := s3.NewFromConfig(*cfg)

	return &S3Storage{
		client:     client,
		bucket:     bucket,
		pathPrefix: pathPrefix,
		endpoint:   endpoint,
	}, nil
}

// Upload uploads a backup directory to S3
func (s *S3Storage) Upload(ctx context.Context, localPath string, backupID string, appName string) (string, error) {
	remotePath := s.GetRemotePath(appName, backupID)

	// Create uploader
	uploader := manager.NewUploader(s.client)

	// Walk through directory and upload files
	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing %s: %w", path, err)
		}

		if info.IsDir() {
			return nil // Don't upload directories themselves
		}

		// Get relative path for S3 key
		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return fmt.Errorf("error computing relative path: %w", err)
		}

			   s3Key := filepath.Join(remotePath, relPath)
			   // Normalize path separators for S3 (always use /)
			   s3Key = strings.ReplaceAll(s3Key, "\\", "/")
			   // Remove any leading dashes or slashes
			   s3Key = strings.TrimLeft(s3Key, "-/")

		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("error opening file %s: %w", path, err)
		}
		defer file.Close()

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(s3Key),
			Body:   file,
			ACL:    types.ObjectCannedACLPrivate, // Keep backups private
		})
		if err != nil {
			return fmt.Errorf("error uploading %s to S3: %w", path, err)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload backup to S3: %w", err)
	}

	return remotePath, nil
}

// Download downloads a backup from S3 to local path
func (s *S3Storage) Download(ctx context.Context, backupID string, remotePath string, localPath string) error {
	// Create local directory
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// List all objects with the remote path prefix
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(remotePath),
	})

	downloader := manager.NewDownloader(s.client)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects in S3: %w", err)
		}

		for _, obj := range page.Contents {
			// Get the relative path (remove prefix)
			relPath := strings.TrimPrefix(*obj.Key, remotePath)
			relPath = strings.TrimPrefix(relPath, "/")

			if relPath == "" {
				continue // Skip the prefix itself
			}

			localFilePath := filepath.Join(localPath, relPath)

			// Create directory if needed
			if err := os.MkdirAll(filepath.Dir(localFilePath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			file, err := os.Create(localFilePath)
			if err != nil {
				return fmt.Errorf("failed to create local file %s: %w", localFilePath, err)
			}

			_, err = downloader.Download(ctx, file, &s3.GetObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			})
			file.Close()

			if err != nil {
				os.Remove(localFilePath) // Clean up partial file
				return fmt.Errorf("failed to download %s from S3: %w", *obj.Key, err)
			}
		}
	}

	return nil
}

// Delete removes a backup from S3
func (s *S3Storage) Delete(ctx context.Context, remotePath string) error {
	// List all objects with the remote path prefix
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
		Prefix: aws.String(remotePath),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return fmt.Errorf("failed to list objects for deletion: %w", err)
		}

		for _, obj := range page.Contents {
			_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
				Bucket: aws.String(s.bucket),
				Key:    obj.Key,
			})
			if err != nil {
				return fmt.Errorf("failed to delete %s from S3: %w", *obj.Key, err)
			}
		}
	}

	return nil
}

// Exists checks if a backup exists in S3
func (s *S3Storage) Exists(ctx context.Context, remotePath string) (bool, error) {
	maxKeys := int32(1)
	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(remotePath),
		MaxKeys: &maxKeys,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to check object existence: %w", err)
		}

		if len(page.Contents) > 0 {
			return true, nil
		}
	}

	return false, nil
}

// List lists all backups for an app in S3
func (s *S3Storage) List(ctx context.Context, appName string) ([]string, error) {
	var backups []string

	prefix := s.GetRemotePath(appName, "")

	paginator := s3.NewListObjectsV2Paginator(s.client, &s3.ListObjectsV2Input{
		Bucket:    aws.String(s.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list backups: %w", err)
		}

		for _, prefix := range page.CommonPrefixes {
			if prefix.Prefix != nil {
				// Extract backup ID from prefix
				backupID := strings.TrimPrefix(*prefix.Prefix, s.GetRemotePath(appName, ""))
				backupID = strings.TrimSuffix(backupID, "/")
				if backupID != "" {
					backups = append(backups, backupID)
				}
			}
		}
	}

	return backups, nil
}

// GetRemotePath returns the S3 key prefix for a backup
func (s *S3Storage) GetRemotePath(appName string, backupID string) string {
	path := appName
	if backupID != "" {
		path = filepath.Join(appName, backupID)
	}
	path = strings.ReplaceAll(path, "\\", "/")

	if s.pathPrefix != "" {
		path = filepath.Join(s.pathPrefix, path)
		path = strings.ReplaceAll(path, "\\", "/")
	}

	// Ensure trailing slash for directory
	if !strings.HasSuffix(path, "/") && backupID != "" {
		path += "/"
	}

	// Remove any leading dashes or slashes
	path = strings.TrimLeft(path, "-/")

	return path
}

// GetS3URL returns the S3 URL for a backup (for reference)
func (s *S3Storage) GetS3URL(remotePath string) string {
	// Format: https://bucket.endpoint/path or https://endpoint/bucket/path
	// Most S3 services use: https://bucket.endpoint/path
	if strings.Contains(s.endpoint, "digitaloceanspaces.com") {
		// DigitalOcean format: https://bucket.nyc3.digitaloceanspaces.com/path
		parts := strings.Split(s.endpoint, "://")
		if len(parts) == 2 {
			return fmt.Sprintf("https://%s.%s/%s", s.bucket, parts[1], remotePath)
		}
	}

	// Generic S3 format: https://endpoint/bucket/path
	endpoint := strings.TrimSuffix(s.endpoint, "/")
	return fmt.Sprintf("%s/%s/%s", endpoint, s.bucket, remotePath)
}
