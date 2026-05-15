# Backup and Restore Guide

This guide explains how to use per-app backup and restore in atmosphere.

## Table of Contents

1. [Overview](#overview)
2. [What Is Backed Up](#what-is-backed-up)
3. [Prerequisites](#prerequisites)
4. [Create App Backup](#create-app-backup)
5. [Check Backup Status](#check-backup-status)
6. [List Backups for an App](#list-backups-for-an-app)
7. [Restore App from Backup](#restore-app-from-backup)
8. [Restore as New App](#restore-as-new-app)
9. [Check Restore Status](#check-restore-status)
10. [Backup Storage Location](#backup-storage-location)
11. [Troubleshooting](#troubleshooting)

---

## Overview

Atmosphere supports **app-level backups**. Each backup is scoped to one app and can be restored for that same app.

Backups and restores run asynchronously:
- API returns immediately with a backup or restore record
- Operation continues in the background
- You can poll status endpoints to see progress

## What Is Backed Up

For each app backup, atmosphere stores:

1. App metadata (`metadata.json`):
- app configuration snapshot
- recent deployment logs

2. App workspace archive (`workspace.tar.gz`):
- files from app workspace directory

3. Deployment key (`deployment.key`) if present:
- app SSH deployment key

4. Docker named volumes (`volumes/*.tar.gz`):
- named volumes attached to containers with label `atmosphere.app=<app-name>`

## Prerequisites

1. Atmosphere API is running (default: `http://localhost:3000`)
2. App already exists in atmosphere
3. Docker is available on the host
4. Sufficient disk space in logs directory

## Create App Backup

Start a backup for one app:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/backups
```

Example response (`202 Accepted`):

```json
{
  "message": "App backup started",
  "backup": {
    "id": 12,
    "backup_id": "my-app-1746659903",
    "app_id": 3,
    "status": "in_progress",
    "path": "/opt/atmosphere/logs/backups/my-app/my-app-1746659903",
    "size_bytes": 0,
    "log": "",
    "started_at": "2026-05-07T10:11:43Z"
  }
}
```

Save `backup.backup_id` for status checks and restore.

## Check Backup Status

Check one backup:

```bash
curl http://localhost:3000/api/v1/apps/my-app/backups/my-app-1746659903
```

Possible status values:
- `in_progress`
- `success`
- `failed`

When complete, `size_bytes`, `log`, and `completed_at` are populated.

## List Backups for an App

List recent backups:

```bash
curl http://localhost:3000/api/v1/apps/my-app/backups
```

Use optional limit:

```bash
curl "http://localhost:3000/api/v1/apps/my-app/backups?limit=50"
```

## Restore App from Backup

Start restore for one app from one successful backup:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "my-app-1746659903"
  }'
```

Optional restore fields:
- `restore_as_new` (boolean)
- `new_app_name` (required when `restore_as_new=true`)
- `strict` (boolean, default `true`)

Restore strictness:
- `strict=true` (default): fail restore on preflight/runtime validation errors
- `strict=false`: continue restore in best-effort mode and record warnings in restore log

Example response (`202 Accepted`):

```json
{
  "message": "App restore started",
  "restore": {
    "id": 7,
    "restore_id": "my-app-1746660020",
    "app_id": 3,
    "backup_id": "my-app-1746659903",
    "status": "in_progress",
    "log": "",
    "started_at": "2026-05-07T10:13:40Z"
  }
}
```

## Restore as New App

Restore one app backup into a separate app name:

```bash
curl -X POST http://localhost:3000/api/v1/apps/openproject/restores \
  -H "Content-Type: application/json" \
  -d '{
    "backup_id": "openproject-1746659903",
    "restore_as_new": true,
    "new_app_name": "openproject-restore",
    "strict": false
  }'
```

Behavior:
- new app record is created
- new app starts with empty domains to avoid routing conflicts
- workspace and key are restored under the new app name
- compose-style volume names are remapped to the new app prefix

## Restore to Fresh Machine from S3

Use this when the destination machine has no existing app record and the backup lives in S3:

```bash
curl -X POST http://localhost:3000/api/v1/restores/fresh \
  -H "Content-Type: application/json" \
  -d '{
    "source_app": "openproject",
    "backup_id": "my-app-1746659903",
    "app_name": "openproject",
    "strict": true
  }'
```

Behavior:
- downloads the backup from storage using `source_app` and `backup_id`
- recreates the app from `metadata.json`
- restores workspace, deployment key, and volumes into the new app
- runs restore preflight checks for workspace/build files before deployment
- validates volume mapping and warns when custom volume names cannot be safely remapped
- deploys from the restored workspace snapshot (no Git clone/fetch/pull)
- verifies post-deploy container runtime state and surfaces unhealthy/exited containers in restore logs
- applies strictness mode (`strict=true` fail-fast, `strict=false` best-effort with warnings)
- keeps the existing app-scoped restore flow unchanged

## Check Restore Status

Check one restore run:

```bash
curl http://localhost:3000/api/v1/apps/my-app/restores/my-app-1746660020
```

Possible status values:
- `in_progress`
- `success`
- `failed`

## Backup Storage Location

Backups are stored on local disk under:

```text
<LOGS_DIR>/backups/<app-name>/<backup-id>/
```

With default settings:

```text
/opt/atmosphere/logs/backups/<app-name>/<backup-id>/
```

Typical structure:

```text
metadata.json
workspace.tar.gz
deployment.key
volumes/
  <volume-archive>.tar.gz
```

## Troubleshooting

### Backup fails with Docker errors

1. Check atmosphere service logs:

```bash
journalctl -u atmosphere -f
```

2. Confirm Docker is healthy:

```bash
docker info
```

### Backup stuck in `in_progress`

1. Check host CPU/disk utilization
2. Check large volume size and available disk space
3. Review backup `log` field from backup detail endpoint

### Restore completes but app is still unhealthy

1. Check app container logs
2. Verify external dependencies (database, DNS, secrets)
3. Trigger a redeploy if your app needs rebuild/restart logic after data restore

### Restore fails during preflight

Common preflight failures:
1. Compose app restore cannot find compose file in restored workspace
2. Dockerfile app restore cannot find Dockerfile path from metadata
3. Restored build directory is missing (for example, wrong `github_subdir` in backup metadata)

Fix:
1. Inspect restore `log` for exact missing path
2. Verify backup `workspace.tar.gz` includes required build files
3. Recreate backup after correcting app build path configuration

### Restore fails on runtime validation

Atmosphere now validates container runtime state after restore deployment.

Validation fails if:
1. no app containers are discovered
2. one or more containers exit immediately
3. one or more containers report non-healthy Docker health status

Fix:
1. Check failing container names in restore `log`
2. Review application logs and env/secret wiring
3. Confirm destination host has required external services reachable

### `backup not found` or `restore not found`

1. Confirm app name is correct
2. Confirm backup/restore ID belongs to the same app
3. Use list endpoint to verify IDs

---

## Notes

1. Current implementation is app-scoped only (one app at a time).
2. Backups are local artifacts; remote storage and retention automation can be added later.
3. Restore supports both in-place mode and `restore_as_new` mode.
