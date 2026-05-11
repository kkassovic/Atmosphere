# S3 Backup & Restore Guide

## Overview

Atmosphere supports flexible backup storage with three options:

1. **S3 Cloud Storage** (DigitalOcean Spaces, AWS S3, MinIO) - offsite disaster recovery
2. **Mounted CIFS/SMB** (NAS, network shares) - local network storage  
3. **Mounted USB Drive** (external storage) - portable local backups

This guide covers S3 setup. For complete comparison and setup of all three options, see [BACKUP_STORAGE_OPTIONS.md](BACKUP_STORAGE_OPTIONS.md).

## Architecture

The backup system is built on a pluggable storage interface:

```
AppService
  ├─ Local Storage (existing)
  └─ S3 Storage (new)
      ├─ Upload backups
      ├─ List remote backups
      └─ Download for restore
```

### Flow

**Backup:**
1. Create backup locally (metadata, workspace, volumes)
2. Upload to S3
3. Store S3 path in database
4. Optionally delete local copy

**Restore:**
1. Check if backup exists locally
2. If not, download from S3 to temporary location
3. Restore (same as before)
4. Clean up temporary files

## Configuration

### Environment Variables

```bash
# S3 Storage (optional, if not set, local storage is used)
S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
S3_BUCKET=your-bucket-name
S3_REGION=nyc3
S3_ACCESS_KEY=YOUR_DO_SPACES_KEY
S3_SECRET_KEY=YOUR_DO_SPACES_SECRET
S3_PATH_PREFIX=atmosphere-backups  # Optional prefix in bucket
```

For systemd installations, these variables must be in `/opt/atmosphere/.env`.
If you prepared `.env` inside your repository directory, copy it to the service location and restart:

```bash
cd ~/atmosphere
cp .env.example .env
nano .env

sudo cp .env /opt/atmosphere/.env
sudo systemctl restart atmosphere
```

### Verify S3 Configuration

Before running backups, verify your S3 connection:

```bash
curl http://localhost:3000/api/v1/backup-storage/health
```

Response:
```json
{
  "status": "ok",
  "backend": "s3",
  "s3_endpoint": "https://nyc3.digitaloceanspaces.com",
  "s3_bucket": "my-backups-bucket",
  "message": "S3 connection successful"
}
```

For detailed health check documentation, see [BACKUP_STORAGE_HEALTH_API.md](BACKUP_STORAGE_HEALTH_API.md).

### DigitalOcean Spaces Setup

1. Create a Space in DigitalOcean console
2. Generate API key (not personal access token)
3. Set endpoints:
   - For NYC: `https://nyc3.digitaloceanspaces.com`
   - For SFO: `https://sfo3.digitaloceanspaces.com`
   - For AMS: `https://ams3.digitaloceanspaces.com`

## Usage

### Backing up to S3

```bash
POST /api/v1/apps/{name}/backups
Body: { "upload_to_s3": true }
```

Response:
```json
{
  "backup_id": "myapp-1234567890",
  "status": "in_progress",
  "s3_path": "nyc3.digitaloceanspaces.com/my-bucket/atmosphere-backups/myapp-1234567890/"
}
```

### Restoring from S3

The restore endpoint automatically detects and downloads from S3 if needed:

```bash
POST /api/v1/apps/{target-app}/restores
Body: {
  "backup_id": "myapp-1234567890",
  "source_app": "original-app-name"  # Required when restoring S3 backup
}
```

The system will:
1. Find the backup (locally or in S3)
2. Download if necessary
3. Restore the application
4. Clean up temporary files

## CLI Examples

### Backup to S3
```bash
curl -X POST http://localhost:3000/api/v1/apps/myapp/backups \
  -H "Content-Type: application/json" \
  -d '{"upload_to_s3": true}'
```

### List backups (including remote)
```bash
curl http://localhost:3000/api/v1/apps/myapp/backups
```

### Get backup details
```bash
curl http://localhost:3000/api/v1/apps/myapp/backups/myapp-1234567890
```

### Restore from S3
```bash
curl -X POST http://localhost:3000/api/v1/apps/target-app/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "myapp-1234567890",
    "source_app": "original-app-name"
  }'
```

### Restore as new app
```bash
curl -X POST http://localhost:3000/api/v1/apps/target-app/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "myapp-1234567890",
    "source_app": "original-app-name",
    "restore_as_new": true,
    "new_app_name": "myapp-copy"
  }'
```

## Future Extensibility

The storage interface supports future implementations:

- **Google Cloud Storage** - Add GCSStorage interface
- **Azure Blob Storage** - Add AzureStorage interface
- **AWS S3** - Same as S3Storage, just different endpoint
- **Hybrid** - Simultaneously backup to multiple storage providers
- **Encryption** - Add encryption layer before upload
- **Compression** - Optimize storage with better compression
- **Retention policies** - Automatic cleanup of old backups

To add a new storage provider:

1. Implement the `BackupStorage` interface
2. Create a factory method in `storage/factory.go`
3. Update config to accept new provider type
4. Update tests and documentation

## Security Considerations

- **Credentials**: Store S3 keys in environment variables, never commit them
- **HTTPS**: Always use HTTPS S3 endpoints
- **Bucket policies**: Restrict bucket access to necessary principals
- **ACLs**: Keep backups private (not public-read)
- **Encryption at rest**: Enable S3 server-side encryption
- **Encryption in transit**: HTTPS + TLS
- **Key rotation**: Periodically rotate S3 credentials

## Troubleshooting

### S3 Connection Issues

```bash
# Test S3 credentials
curl -v https://your-bucket.nyc3.digitaloceanspaces.com/
```

### Download Failures

Check logs for:
- Network connectivity to S3
- Correct credentials
- Bucket permissions

### Storage Space

Monitor S3 usage:
```bash
# Via DigitalOcean CLI
doctl compute spaces list-contents your-bucket
```

## Backup Retention

Implement retention policies:

```bash
# Keep only last 10 backups per app (future feature)
DELETE FROM app_backups 
WHERE app_id = ? 
AND backup_id NOT IN (
  SELECT backup_id FROM app_backups 
  WHERE app_id = ? 
  ORDER BY created_at DESC 
  LIMIT 10
);
```

## Example: Cross-Machine Deployment

1. **Machine A** - Backup app to S3:
```bash
curl -X POST http://machine-a:3000/api/v1/apps/myapp/backups \
  -H "Content-Type: application/json" \
  -d '{"upload_to_s3": true}'
```

2. **Machine B** - Restore from S3:
```bash
curl -X POST http://machine-b:3000/api/v1/apps/myapp/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "myapp-1234567890",
    "source_app": "myapp"
  }'
```

Done! Machine B now has the exact state of Machine A's app.
