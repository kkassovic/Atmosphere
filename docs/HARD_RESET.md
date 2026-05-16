# Hard Reset

Hard reset wipes all Atmosphere-managed data in a single operation: containers, Docker volumes, workspaces, deployment keys, logs, local backup archives, and the SQLite database.

Two things are **never** touched:
- `*.ini` files inside managed directories — collected before wipe and restored after
- S3 backups — no S3 API calls are made

After a hard reset the Atmosphere server must be restarted to initialise a fresh database.

---

## API

```
POST /api/v1/system/hard-reset
Content-Type: application/json

{"confirm": true}
```

The `"confirm": true` field is required. The request is rejected without it.

### Example

```bash
curl -X POST http://localhost:3000/api/v1/system/hard-reset \
  -H "Content-Type: application/json" \
  -d '{"confirm": true}'
```

### Response

```json
{
  "containers_removed": 4,
  "volumes_removed": 6,
  "dirs_wiped": [
    "/opt/atmosphere/workspaces",
    "/opt/atmosphere/keys",
    "/opt/atmosphere/logs"
  ],
  "database_deleted": true,
  "ini_files_preserved": 0,
  "errors": []
}
```

If some cleanup steps fail (e.g. a container that can't be removed), the errors are collected in the `errors` array and the reset continues. The response is always `200 OK` as long as the request was valid.

### Response Fields

| Field | Type | Description |
|-------|------|-------------|
| `containers_removed` | int | Number of containers successfully removed |
| `volumes_removed` | int | Number of Docker volumes removed |
| `dirs_wiped` | []string | Directories whose contents were deleted |
| `database_deleted` | bool | Whether the SQLite database file was deleted |
| `ini_files_preserved` | int | Number of `*.ini` files that were preserved |
| `errors` | []string | Non-fatal errors (reset continued despite these) |

### HTTP Status Codes

| Status | Meaning |
|--------|---------|
| `200 OK` | Reset ran (check `errors` for partial failures) |
| `400 Bad Request` | `{"confirm": true}` not provided |
| `500 Internal Server Error` | Unexpected failure before reset started |

---

## CLI

```bash
atmosphere-cli system hard-reset --confirm
```

`--confirm` is required. Without it the command exits with an error.

```bash
# With custom API URL
atmosphere-cli --api http://localhost:3000 system hard-reset --confirm
```

---

## What Gets Deleted

| Resource | Deleted |
|----------|---------|
| App containers (label `atmosphere.app`) | Yes — force-removed |
| Docker volumes from `atmosphere-*` compose projects | Yes |
| `/opt/atmosphere/workspaces/**` | Yes |
| `/opt/atmosphere/keys/**` | Yes |
| `/opt/atmosphere/logs/**` (including local backup archives) | Yes |
| `/opt/atmosphere/atmosphere.db` | Yes |
| `*.ini` files inside managed directories | **No — preserved** |
| S3 backup objects | **No — never touched** |
| Traefik configuration (`/opt/traefik`) | **No** |
| Atmosphere binaries (`atmosphere`, `atmosphere-cli`) | **No** |
| `.env` configuration file | **No** |

---

## After a Hard Reset

Restart the Atmosphere service to initialise a fresh database and recreate the required directories:

```bash
systemctl restart atmosphere
```

Verify the server is up:

```bash
curl http://localhost:3000/health
# {"status":"ok"}
```

The instance is now clean. You can create new apps from scratch or restore from S3 backups using the fresh-restore flow:

```bash
atmosphere-cli restores fresh \
  --source-app my-app \
  --backup-id my-app-1778692456
```

---

## Comparison: Delete vs Destroy vs Hard Reset

| Operation | Scope | Preserves backups |
|-----------|-------|-------------------|
| `DELETE /apps/{name}` | Single app — containers + DB record | Yes |
| `POST /apps/{name}/destroy` | Single app — full wipe, keeps backup records | Yes |
| `POST /system/hard-reset` | Entire instance — everything | S3 only |
