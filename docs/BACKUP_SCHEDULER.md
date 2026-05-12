# Backup Scheduler Guide

This guide explains how per-app scheduled backups work in atmosphere.

## Table of Contents

1. [Overview](#overview)
2. [How It Works](#how-it-works)
3. [API](#api)
4. [Schedule Fields](#schedule-fields)
5. [Operational Notes](#operational-notes)
6. [Recommended Usage](#recommended-usage)

---

## Overview

Atmosphere supports **per-app recurring backups**.

Each app can have its own schedule stored in the Atmosphere database. The scheduler runs inside the Atmosphere backend process, polls for due schedules, and reuses the existing backup service to create backups.

This keeps the schedule logic inside the Atmosphere repo and avoids a separate cron-only control plane.

## How It Works

1. You create or update a backup schedule for an app.
2. Atmosphere stores the schedule in `app_backup_schedules`.
3. The scheduler loop wakes up periodically and finds due schedules.
4. For each due schedule, Atmosphere calls the same backup path used by manual backups.
5. The schedule record is updated with the last run status and next run time.

Backups created by the scheduler are still normal Atmosphere backups, so they can be listed, inspected, uploaded to S3, and restored the same way as manual backups.

## API

### Get one app's backup schedule

```bash
curl http://localhost:3000/api/v1/apps/my-app/backup-schedule
```

### Create or update one app's backup schedule

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app/backup-schedule \
  -H "Content-Type: application/json" \
  -d '{
    "enabled": true,
    "interval_minutes": 1440,
    "upload_to_s3": true
  }'
```

Example response:

```json
{
  "id": 1,
  "app_id": 3,
  "enabled": true,
  "interval_minutes": 1440,
  "upload_to_s3": true,
  "last_backup_id": "my-app-1746659903",
  "last_run_at": "2026-05-12T10:00:00Z",
  "next_run_at": "2026-05-13T10:00:00Z",
  "last_status": "queued",
  "last_error": "",
  "created_at": "2026-05-12T09:00:00Z",
  "updated_at": "2026-05-12T10:00:00Z"
}
```

## Schedule Fields

- `enabled`: turns the schedule on or off.
- `interval_minutes`: how often Atmosphere should create a backup.
- `upload_to_s3`: whether scheduled backups should also be uploaded to S3.
- `last_backup_id`: most recent backup ID created by the scheduler.
- `last_run_at`: when the scheduler last processed the app.
- `next_run_at`: next due time for the app.
- `last_status`: latest scheduler outcome.
- `last_error`: latest scheduler error message if one occurred.

## Operational Notes

- The scheduler is interval-based, not cron-expression-based.
- One Atmosphere instance should own the scheduler loop for a given database.
- The scheduler reuses the existing backup service, so there is no duplicate backup implementation.
- Manual backups and scheduled backups share the same backup storage layout.
- If a backup takes longer than the interval, the next run is still driven by the persisted `next_run_at` value.

## Recommended Usage

For most installs, start with one daily backup per app:

- `interval_minutes: 1440` for daily backups
- `upload_to_s3: true` if you want offsite copies
- Keep manual backups enabled for one-off snapshots before risky changes

For apps with active data changes, consider shorter intervals only if your storage and backup time stay comfortably below the interval length.
