# Atmosphere Cheat Sheet

Copy-paste commands for common Atmosphere app operations.

## CLI Defaults and Helpers

```bash
# Optional: shorten command
alias atm='atmosphere-cli'

# Optional: avoid repeating API URL
export ATM_API="http://localhost:3000"

# Global flags examples
atmosphere-cli --api "$ATM_API" apps list
atmosphere-cli --api "$ATM_API" --timeout 60s apps logs openp --limit 50
```

---

## Create App (CLI JSON inline)

```bash
atmosphere-cli apps create --json "$(jq -n \
  --arg name "openp" \
  --arg deployment_type "github" \
  --arg build_type "compose" \
  --arg compose_file "docker-compose.prod.yml" \
  --arg github_repo "git@github.com:kkassovic/openproject.git" \
  --arg github_branch "main" \
  --arg deployment_key "$(cat ~/.ssh/open2-key)" \
  --arg domain "opentest.kassovicms.com" \
  '{
    name: $name,
    deployment_type: $deployment_type,
    build_type: $build_type,
    compose_path: $compose_file,
    github_repo: $github_repo,
    github_branch: $github_branch,
    deployment_key: $deployment_key,
    domains: [$domain],
    env_vars: {
      NODE_ENV: "production"
    }
  }')"
```

## Create App (from file)

```bash
cat > app-openp.json <<'JSON'
{
  "name": "openp",
  "deployment_type": "github",
  "build_type": "compose",
  "compose_path": "docker-compose.prod.yml",
  "github_repo": "git@github.com:kkassovic/openproject.git",
  "github_branch": "main",
  "deployment_key": "REPLACE_WITH_PRIVATE_KEY_CONTENT",
  "domains": ["opentest.kassovicms.com"],
  "env_vars": {
    "NODE_ENV": "production"
  }
}
JSON

atmosphere-cli apps create --file app-openp.json
```

## Create App (GitHub + Compose + Domain)

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d "$(jq -n \
    --arg name "openp" \
    --arg deployment_type "github" \
    --arg build_type "compose" \
    --arg compose_file "docker-compose.prod.yml" \
    --arg github_repo "git@github.com:kkassovic/openproject.git" \
    --arg github_branch "main" \
    --arg deployment_key "$(cat ~/.ssh/open2-key)" \
    --arg domain "opentest.kassovicms.com" \
    '{
      name: $name,
      deployment_type: $deployment_type,
      build_type: $build_type,
      compose_path: $compose_file,
      github_repo: $github_repo,
      github_branch: $github_branch,
      deployment_key: $deployment_key,
      domains: [$domain],
      env_vars: {
        NODE_ENV: "production"
      }
    }')"
```

---

## List Apps

```bash
atmosphere-cli apps list
curl -s http://localhost:3000/api/v1/apps | jq -r '.[] | "\(.name): \(.status)"'
```

## App Details

```bash
atmosphere-cli apps get openp
curl -s http://localhost:3000/api/v1/apps/openp | jq
```

---

## Deploy

```bash
atmosphere-cli apps deploy openp
curl -X POST http://localhost:3000/api/v1/apps/openp/deploy
```

## Stop and Start

```bash
atmosphere-cli apps stop openp
atmosphere-cli apps start openp
```

## Delete vs Destroy

```bash
# Delete app record/container resources (standard delete)
atmosphere-cli apps delete openp

# Deep wipe app resources while preserving backups
atmosphere-cli apps destroy openp
```

## Update App (CLI)

```bash
atmosphere-cli apps update reopen3 --json '{
  "env_vars": {
    "NODE_ENV": "production"
  }
}'
```

---

## Deploy Status and Logs

```bash
# Latest 20 logs via CLI
atmosphere-cli apps logs openp --limit 20

# Latest deployment status
curl -s http://localhost:3000/api/v1/apps/openp/logs | jq -r '.[0].status'

# Latest deployment log text
curl -s http://localhost:3000/api/v1/apps/openp/logs | jq -r '.[0].log'

# Full deployment logs (progress view)
curl http://localhost:3000/api/v1/apps/openp/logs | jq
```

---

## Backups (CLI)

```bash
# Create local backup
atmosphere-cli backups create openp

# Create backup and upload to S3
atmosphere-cli backups create openp --upload-to-s3

# List and inspect backups
atmosphere-cli backups list openp --limit 20
atmosphere-cli backups get openp openp-1778692456

# Delete a specific backup
atmosphere-cli backups delete openp openp-1778692456
```

---

## Get Merged Compose Configuration

```bash
curl http://localhost:3000/api/v1/apps/openp/compose-config
```

---

## Restore (CLI)

```bash
# Restore into existing app context
atmosphere-cli restores start openp --backup-id openp-1778692456

# Restore as a new app from existing app backup set
atmosphere-cli restores start openp \
  --backup-id openp-1778692456 \
  --restore-as-new \
  --new-app-name reopen2

# Fresh restore flow (skip git sync)
atmosphere-cli restores fresh \
  --source-app openproject \
  --backup-id openproject-1778692456 \
  --app-name reopen3

# Get restore operation details
atmosphere-cli restores get openp openp-1778692600
```

---

## Verify S3 Configuration

```bash
curl http://localhost:3000/api/v1/backup-storage/health | jq
```

---

## Templates (CLI)

```bash
# List available templates
atmosphere-cli templates list

# Inspect one template
atmosphere-cli templates get wordpress

# Provision template from JSON file
atmosphere-cli templates provision wordpress --file provision.json
```

---

## Restore to Fresh Machine from S3

```bash
curl -X POST http://localhost:3000/api/v1/restores/fresh \
  -H "Content-Type: application/json" \
  -d '{
    "source_app": "openproject",
    "backup_id": "openproject-1778692456",
    "app_name": "reopen2"
  }'

curl -X POST http://localhost:3000/api/v1/restores/fresh \
  -H "Content-Type: application/json" \
  -d '{
    "source_app": "openproject",
    "backup_id": "openproject-1778692456",
    "app_name": "reopen3"
  }'
```

---

## Update App

```bash
curl -X PUT http://localhost:3000/api/v1/apps/reopen3 \
  -H "Content-Type: application/json" \
  -d '{
      "env_vars": {
      "NODE_ENV": "production"
    }
  }'
```

---

## Hard Reset (Wipe Everything)

Permanently deletes all containers, volumes, workspaces, keys, logs, local backups, and the database. `*.ini` files and S3 backups are preserved. The server must be restarted afterwards.

```bash
# CLI
atmosphere-cli system hard-reset --confirm

# API
curl -X POST http://localhost:3000/api/v1/system/hard-reset \
  -H "Content-Type: application/json" \
  -d '{"confirm": true}'

# Restart the server after reset
systemctl restart atmosphere
```
