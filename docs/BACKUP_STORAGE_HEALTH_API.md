# Backup Storage Health Check API

## Endpoint

```
GET /api/v1/backup-storage/health
```

## Purpose

Verify that your configured backup storage (S3, local filesystem, CIFS, USB) is accessible and working properly. Useful for:

- Debugging configuration issues
- Pre-deployment validation
- Monitoring storage health
- Testing credentials before using for backups

## Response - Success

### With S3 Configured

```bash
curl http://localhost:3000/api/v1/backup-storage/health
```

```json
{
  "status": "ok",
  "backend": "s3",
  "s3_endpoint": "https://nyc3.digitaloceanspaces.com",
  "s3_bucket": "my-backups-bucket",
  "message": "S3 connection successful"
}
```

### With Local Storage (Default)

```json
{
  "status": "ok",
  "backend": "local",
  "logs_dir": "/opt/atmosphere/logs",
  "message": "Local storage accessible"
}
```

## Response - Issues

### S3 Connection Failed

```json
{
  "status": "warning",
  "backend": "s3",
  "s3_endpoint": "https://nyc3.digitaloceanspaces.com",
  "s3_bucket": "my-backups-bucket",
  "s3_error": "InvalidAccessKeyId: The AWS Access Key Id you provided does not exist in our records",
  "message": "S3 configured but not accessible"
}
```

### Local Storage Missing

```json
{
  "status": "warning",
  "backend": "local",
  "message": "Logs directory does not exist: /opt/atmosphere/logs"
}
```

### Local Storage Permission Issues

```json
{
  "status": "warning",
  "backend": "local",
  "error": "permission denied",
  "logs_dir": "/opt/atmosphere/logs"
}
```

## HTTP Status Codes

- `200 OK` - Storage is accessible (check `status` field for details)
- `500 Internal Server Error` - Unexpected error

## Testing Examples

### Test with cURL

```bash
# Test backup storage health
curl -X GET http://localhost:3000/api/v1/backup-storage/health

# Pretty print the response
curl -X GET http://localhost:3000/api/v1/backup-storage/health | jq .
```

### Test with Python

```python
import requests
import json

response = requests.get('http://localhost:3000/api/v1/backup-storage/health')
health = response.json()

print(f"Status: {health['status']}")
print(f"Backend: {health['backend']}")
print(f"Message: {health['message']}")

if health['status'] != 'ok':
    print(f"Warning: {health.get('error', health.get('s3_error', 'Unknown error'))}")
```

### Test with Bash

```bash
#!/bin/bash

response=$(curl -s http://localhost:3000/api/v1/backup-storage/health)
status=$(echo $response | jq -r '.status')
backend=$(echo $response | jq -r '.backend')

if [ "$status" = "ok" ]; then
  echo "✅ Backup storage is healthy ($backend)"
else
  echo "⚠️ Backup storage issue:"
  echo $response | jq .
fi
```

## Troubleshooting

### "S3 configured but not accessible"

Check:
1. S3 credentials are correct
2. Bucket name is correct
3. S3 endpoint is correct
4. Bucket exists and is accessible from your network
5. IAM user has s3:ListBucket permission

```bash
# Test with AWS CLI
aws s3 ls --region nyc3 --endpoint-url https://nyc3.digitaloceanspaces.com
```

### "Logs directory does not exist"

Create the directory:

```bash
mkdir -p /opt/atmosphere/logs
chmod 755 /opt/atmosphere/logs
```

### "Permission denied"

Fix directory permissions:

```bash
chmod 755 /opt/atmosphere
chmod 755 /opt/atmosphere/logs
chown atmosphere:atmosphere /opt/atmosphere/logs  # If running as specific user
```

## Integration with Monitoring

### Prometheus Metrics (Future Enhancement)

Track backup storage health status over time:

```bash
# Scrape endpoint
curl http://localhost:3000/api/v1/backup-storage/health
```

### Nagios/Icinga Check

```bash
#!/bin/bash
# check_atmosphere_storage.sh

RESPONSE=$(curl -s http://localhost:3000/api/v1/backup-storage/health)
STATUS=$(echo $RESPONSE | jq -r '.status')
BACKEND=$(echo $RESPONSE | jq -r '.backend')
MESSAGE=$(echo $RESPONSE | jq -r '.message')

if [ "$STATUS" = "ok" ]; then
  echo "OK: $MESSAGE (Backend: $BACKEND)"
  exit 0
else
  echo "WARNING: $MESSAGE (Backend: $BACKEND)"
  exit 1
fi
```

### Docker Healthcheck

```yaml
services:
  atmosphere:
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:3000/api/v1/backup-storage/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

## Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | "ok" or "warning" |
| `backend` | string | "s3" or "local" |
| `message` | string | Human-readable status message |
| `s3_endpoint` | string | S3 endpoint (S3 only) |
| `s3_bucket` | string | S3 bucket name (S3 only) |
| `s3_error` | string | S3-specific error (if any) |
| `logs_dir` | string | Local logs directory path (local only) |
| `error` | string | General error message |

