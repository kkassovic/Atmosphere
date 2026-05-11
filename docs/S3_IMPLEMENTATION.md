# S3 Backup Implementation - Architecture & Code Changes

## Overview

This document details the S3 backup system implementation for Atmosphere, enabling robust backup storage on DigitalOcean S3 (or any S3-compatible service) and cross-machine disaster recovery.

## Architecture

### Storage Abstraction Layer

The implementation uses an interface-based design allowing multiple storage backends:

```go
type BackupStorage interface {
    Upload(ctx, localPath, backupID, appName) (remotePath, error)
    Download(ctx, backupID, remotePath, localPath) error
    Delete(ctx, remotePath) error
    Exists(ctx, remotePath) (bool, error)
    List(ctx, appName) ([]string, error)
    GetRemotePath(appName, backupID) string
}
```

### Implementations

1. **LocalStorage** - Wraps filesystem backups (default)
2. **S3Storage** - S3-compatible services (DigitalOcean, AWS, MinIO)

Future extensions can add:
- GoogleCloudStorage
- AzureBlobStorage
- Encrypted backups
- Compressed backups

## Code Changes

### 1. New Files Created

#### `backend/internal/storage/storage.go`
- Defines `BackupStorage` interface
- Defines supporting types: `UploadResult`, `DownloadProgress`

#### `backend/internal/storage/s3.go`
- Implements S3Storage using AWS SDK v2
- Handles multipart uploads/downloads
- Compatible with DigitalOcean Spaces, AWS S3, MinIO
- Features:
  - Private ACLs for backups
  - Recursive directory upload
  - Efficient streaming download
  - List and delete operations

#### `backend/internal/storage/local.go`
- Implements LocalStorage for filesystem backups
- Wraps existing backup directory structure
- No-op for upload/download (backups already on disk)

#### `backend/internal/storage/factory.go`
- Factory function to instantiate correct storage backend
- Configuration-driven selection

### 2. Modified Files

#### `backend/go.mod`
Added dependencies:
```
github.com/aws/aws-sdk-go-v2 v1.24.0
github.com/aws/aws-sdk-go-v2/credentials v1.16.12
github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.15.7
github.com/aws/aws-sdk-go-v2/service/s3 v1.47.5
```

#### `backend/internal/config/config.go`
Added S3 configuration fields:
```go
S3Endpoint   string
S3Bucket     string
S3Region     string
S3AccessKey  string
S3SecretKey  string
S3PathPrefix string
```

Added method:
```go
func (c *Config) IsS3Enabled() bool
```

#### `backend/internal/models/models.go`
Extended `AppBackup` struct:
```go
S3Path       string     // Remote S3 path
UploadedToS3 bool       // Whether in S3
S3UploadedAt *time.Time // When uploaded
```

Added `CreateAppBackupRequest`:
```go
type CreateAppBackupRequest struct {
    UploadToS3 bool // Upload to S3 after backup
}
```

Extended `CreateAppRestoreRequest`:
```go
SourceApp string // Original app name for S3 backups
```

#### `backend/internal/services/app_service.go`
- Added `backupStorage` field
- Updated `NewAppService` to accept storage backend
- Updated method signatures to use storage

#### `backend/internal/services/backup_service.go`
- `CreateAppBackup(name string, uploadToS3 bool)` - New uploadToS3 parameter
- `runAppBackup(app, backup, uploadToS3 bool)` - Added S3 upload logic
  - After successful local backup, uploads to S3 if requested
  - Updates backup record with S3 path and timestamp
  - Non-fatal errors logged but don't fail backup
- `StartAppRestore(name, backupID, sourceApp string, ...)` - New sourceApp parameter
  - Allows restoring backups from other apps
  - Required for cross-machine S3 restores
- `runAppRestore` - Enhanced to download from S3
  - Checks if backup exists locally
  - Downloads from S3 if needed
  - Uses temporary directory for S3 downloads
  - Cleans up temporary files after restore

#### `backend/internal/api/handlers.go`
- `CreateAppBackup` - Now reads request body with `UploadToS3` flag
- `StartAppRestore` - Passes `sourceApp` parameter to service

#### `backend/internal/api/routes.go`
- Added imports for storage package
- Initializes S3 storage if configured
- Passes storage to AppService
- Graceful fallback to local storage if S3 init fails

#### `backend/internal/database/migrations.go`
- Added `migrateAppBackupsForS3` function
- Adds S3 columns via `ALTER TABLE`:
  - `s3_path TEXT DEFAULT ''`
  - `uploaded_to_s3 BOOLEAN DEFAULT 0`
  - `s3_uploaded_at DATETIME`
- Handles both new installations and existing databases

#### `backend/internal/repository/app_repository.go`
- Updated `CreateAppBackup` - Inserts S3 fields
- Updated `UpdateAppBackup` - Updates S3 fields
- Updated `GetAppBackupByBackupID` - Scans S3 fields
- Updated `ListAppBackups` - Scans S3 fields

## Configuration

### Environment Variables

```bash
# S3 Backup Storage (optional)
S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
S3_BUCKET=my-backups-bucket
S3_REGION=nyc3
S3_ACCESS_KEY=<your-access-key>
S3_SECRET_KEY=<your-secret-key>
S3_PATH_PREFIX=atmosphere-backups
```

If not set, system defaults to local storage.

## Data Flow

### Backup to S3

1. User calls `POST /api/v1/apps/{name}/backups` with `{"upload_to_s3": true}`
2. Handler creates backup record with status="in_progress"
3. Service starts async `runAppBackup` with uploadToS3=true
4. Backup creates local directory structure
5. Backup collects metadata, workspace, volumes, keys
6. On success:
   - Calculates size
   - Uploads directory tree to S3
   - Updates backup record with S3 path and timestamp
   - Marks status="success"
7. Response includes S3 path for reference

### Restore from S3

1. User calls `POST /api/v1/apps/{target}/restores` with:
   - `backup_id`: (required)
   - `source_app`: (optional, for S3 backups)
   - `restore_as_new`: (optional)
   - `new_app_name`: (optional)
2. Handler passes to service with sourceApp parameter
3. Service looks up backup:
   - First in target app's backups
   - Then in source app's backups (if provided)
4. `runAppRestore` checks if backup exists locally
5. If not found and `uploaded_to_s3=true`:
   - Creates temporary directory
   - Downloads from S3
   - Proceeds with restore
   - Cleans up temp files
6. Restore proceeds as normal
7. Response includes restore status and log

## Security

### Implemented

- Private ACLs on S3 objects (not public-read)
- Credentials via environment variables (never committed)
- HTTPS S3 endpoints only
- TLS encryption in transit

### Recommended

- Enable S3 server-side encryption (SSE)
- Restrict bucket access with IAM/bucket policies
- Enable bucket versioning for recovery
- Rotate credentials periodically
- Use separate S3 keys for Atmosphere
- Enable S3 access logging
- Optionally add client-side encryption (future)

## Testing the Implementation

### Manual Testing

1. **Create backup to S3**:
```bash
curl -X POST http://localhost:3000/api/v1/apps/myapp/backups \
  -H "Content-Type: application/json" \
  -d '{"upload_to_s3": true}'
```

2. **Monitor progress**:
```bash
curl http://localhost:3000/api/v1/apps/myapp/backups/myapp-1234567890
```

3. **Restore from S3** (different machine or app):
```bash
curl -X POST http://localhost:3000/api/v1/apps/myapp-restored/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "myapp-1234567890",
    "source_app": "myapp"
  }'
```

### Verify S3 Upload

```bash
# List objects in bucket
aws s3 ls s3://my-bucket/atmosphere-backups/ --recursive

# Or with DigitalOcean CLI
doctl compute spaces list-contents my-bucket atmosphere-backups/
```

## Performance Characteristics

### Upload
- Streaming multipart upload via AWS SDK
- Memory-efficient for large backups
- Speed depends on network and file count

### Download
- Streaming from S3
- Creates temporary files during restore
- Cleaned up automatically

### Optimization opportunities
- Incremental backups (track changed files)
- Compression before upload
- Parallel uploads for large backups
- Bandwidth throttling

## Error Handling

- S3 upload failures don't fail backup (logged as warning)
- S3 connection failures fall back to local storage
- Missing local/remote backups return clear errors
- Restore downloads fail the entire restore (expected)
- Temporary files cleaned up on error

## Future Extensions

1. **Multi-cloud**: Backup simultaneously to S3 and GCS
2. **Encryption**: Client-side encryption before upload
3. **Retention**: Automatic cleanup of old backups
4. **Incremental**: Only backup changed files
5. **Compression**: Reduce storage size
6. **Scheduling**: Automatic periodic backups
7. **Versioning**: Keep multiple backup versions
8. **Deduplication**: Share common files across backups
