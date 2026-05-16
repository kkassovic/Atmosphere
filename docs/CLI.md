# Atmosphere CLI Guide

This CLI is a thin wrapper around Atmosphere API endpoints.

## Install

The standard atmosphere installer builds and links `atmosphere-cli` automatically, so it is ready to use right after installation:

```bash
atmosphere-cli --help
```

## Build

From the `backend` directory:

```bash
go build -o atmosphere-cli ./cmd/atmosphere-cli
```

Optional install for local development:

```bash
go install ./cmd/atmosphere-cli
```

Why `atmosphere-cli` and not `atmosphere`?

- `backend/cmd/atmosphere` already builds the API server binary named `atmosphere`.
- The CLI uses `atmosphere-cli` to avoid colliding with the server executable.

If you want a shorter command, create an alias:

```bash
alias atm='atmosphere-cli'
atm apps list
```

## Global Flags

- `--api` API base URL (default: `http://localhost:3000`)
- `--timeout` HTTP timeout (default: `30s`)

Examples:

```bash
atmosphere-cli --api http://localhost:3000 apps list
atmosphere-cli --timeout 60s apps logs my-app --limit 50
```

## App Commands

```bash
atmosphere-cli apps list
atmosphere-cli apps get my-app
atmosphere-cli apps create --file app.json
atmosphere-cli apps update my-app --json '{"domains":["app.example.com"]}'
atmosphere-cli apps deploy my-app
atmosphere-cli apps stop my-app
atmosphere-cli apps start my-app
atmosphere-cli apps delete my-app
atmosphere-cli apps destroy my-app
atmosphere-cli apps logs my-app --limit 20
```

### Sample app.json for custom docker-compose override

Use this when your repository follows the base + override compose pattern, for example `docker-compose.yml` plus `docker-compose.prod.yml`:

```json
{
	"name": "my-github-app",
	"deployment_type": "github",
	"build_type": "compose",
	"compose_path": "docker-compose.prod.yml",
	"github_repo": "git@github.com:username/repository.git",
	"github_branch": "main",
	"deployment_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI...",
	"domains": ["app.example.com"],
	"env_vars": {
		"NODE_ENV": "production",
		"DATABASE_URL": "postgresql://..."
	}
}
```

Then create the app with:

```bash
atmosphere-cli apps create --file app.json
```

Or create it inline without a separate file:

```bash
atmosphere-cli apps create --json "$(jq -n \
	--arg name "my-github-app" \
	--arg deployment_type "github" \
	--arg build_type "compose" \
	--arg compose_path "docker-compose.prod.yml" \
	--arg github_repo "git@github.com:username/repository.git" \
	--arg github_branch "main" \
	--arg deployment_key "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAI..." \
	--arg domain "app.example.com" \
	'{
		name: $name,
		deployment_type: $deployment_type,
		build_type: $build_type,
		compose_path: $compose_path,
		github_repo: $github_repo,
		github_branch: $github_branch,
		deployment_key: $deployment_key,
		domains: [$domain],
		env_vars: {
			NODE_ENV: "production",
			DATABASE_URL: "postgresql://..."
		}
	}')"
```

### Sample app.json for manual compose override

Use this when you are creating the app manually but still want Atmosphere to deploy a compose stack that uses an override file:

```json
{
	"name": "my-compose-app",
	"deployment_type": "manual",
	"build_type": "compose",
	"compose_path": "docker-compose.prod.yml",
	"domain": "app.example.com",
	"env_vars": {
		"NODE_ENV": "production",
		"PORT": "3000"
	}
}
```

Then create the app with:

```bash
atmosphere-cli apps create --file app.json
```

Or create it inline without a separate file:

```bash
atmosphere-cli apps create --json "$(jq -n \
	--arg name "my-compose-app" \
	--arg deployment_type "manual" \
	--arg build_type "compose" \
	--arg compose_path "docker-compose.prod.yml" \
	--arg domain "app.example.com" \
	'{
		name: $name,
		deployment_type: $deployment_type,
		build_type: $build_type,
		compose_path: $compose_path,
		domain: $domain,
		env_vars: {
			NODE_ENV: "production",
			PORT: "3000"
		}
	}')"
```

Notes:
- `apps delete` calls `DELETE /api/v1/apps/{name}`.
- `apps destroy` calls `POST /api/v1/apps/{name}/destroy` and performs deep wipe while preserving backups.

## Backup Commands

```bash
atmosphere-cli backups create my-app
atmosphere-cli backups create my-app --upload-to-s3
atmosphere-cli backups list my-app --limit 50
atmosphere-cli backups get my-app my-app-1746659903
atmosphere-cli backups delete my-app my-app-1746659903
```

## Restore Commands

```bash
atmosphere-cli restores start my-app --backup-id my-app-1746659903
atmosphere-cli restores start my-app --backup-id my-app-1746659903 --restore-as-new --new-app-name my-app-restore
atmosphere-cli restores fresh --source-app my-app --backup-id my-app-1746659903 --app-name my-app
atmosphere-cli restores get my-app my-app-1746660020
```

Note: `restores fresh` deploys from the restored workspace snapshot and skips Git sync (no clone/fetch/pull).

## Template Commands

```bash
atmosphere-cli templates list
atmosphere-cli templates get wordpress
atmosphere-cli templates provision wordpress --file provision.json
```

## Template Commands

```bash
atmosphere-cli templates list
atmosphere-cli templates get wordpress
atmosphere-cli templates provision wordpress --file provision.json
```

## System Commands

```bash
# Hard reset — permanently wipes all Atmosphere-managed data
atmosphere-cli system hard-reset --confirm
```

`--confirm` is mandatory. Without it the command refuses to run.

**What hard reset deletes:**
- All containers bearing the `atmosphere.app` label (stopped and force-removed)
- All named Docker volumes from `atmosphere-*` compose projects
- Contents of `WorkspacesDir` (`/opt/atmosphere/workspaces`)
- Contents of `KeysDir` (`/opt/atmosphere/keys`)
- Contents of `LogsDir` (`/opt/atmosphere/logs`) — including local backup archives
- The SQLite database (`/opt/atmosphere/atmosphere.db`)

**What is preserved:**
- Any `*.ini` files found inside managed directories
- S3 backups — never touched

**After a hard reset** the Atmosphere server must be restarted. It will recreate the database and empty directories on next startup:
```bash
systemctl restart atmosphere
```

## JSON Input

For commands that send a JSON body, use one of:

- `--json '<inline-json>'`
- `--file path/to/payload.json`

Exactly one must be provided.

## Behavior

- Successful responses are printed as formatted JSON.
- Non-2xx responses return a non-zero exit code and print API error details.
