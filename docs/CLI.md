# Atmosphere CLI Guide

This CLI is a thin wrapper around Atmosphere API endpoints.

## Build

From the `backend` directory:

```bash
go build -o atmosphere-cli ./cmd/atmosphere-cli
```

Optional install:

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
./atmosphere-cli --api http://localhost:3000 apps list
./atmosphere-cli --timeout 60s apps logs my-app --limit 50
```

## App Commands

```bash
./atmosphere-cli apps list
./atmosphere-cli apps get my-app
./atmosphere-cli apps create --file app.json
./atmosphere-cli apps update my-app --json '{"domains":["app.example.com"]}'
./atmosphere-cli apps deploy my-app
./atmosphere-cli apps stop my-app
./atmosphere-cli apps start my-app
./atmosphere-cli apps delete my-app
./atmosphere-cli apps destroy my-app
./atmosphere-cli apps logs my-app --limit 20
```

Notes:
- `apps delete` calls `DELETE /api/v1/apps/{name}`.
- `apps destroy` calls `POST /api/v1/apps/{name}/destroy` and performs deep wipe while preserving backups.

## Backup Commands

```bash
./atmosphere-cli backups create my-app
./atmosphere-cli backups create my-app --upload-to-s3
./atmosphere-cli backups list my-app --limit 50
./atmosphere-cli backups get my-app my-app-1746659903
./atmosphere-cli backups delete my-app my-app-1746659903
```

## Restore Commands

```bash
./atmosphere-cli restores start my-app --backup-id my-app-1746659903
./atmosphere-cli restores start my-app --backup-id my-app-1746659903 --restore-as-new --new-app-name my-app-restore
./atmosphere-cli restores fresh --source-app my-app --backup-id my-app-1746659903 --app-name my-app
./atmosphere-cli restores get my-app my-app-1746660020
```

## Template Commands

```bash
./atmosphere-cli templates list
./atmosphere-cli templates get wordpress
./atmosphere-cli templates provision wordpress --file provision.json
```

## JSON Input

For commands that send a JSON body, use one of:

- `--json '<inline-json>'`
- `--file path/to/payload.json`

Exactly one must be provided.

## Behavior

- Successful responses are printed as formatted JSON.
- Non-2xx responses return a non-zero exit code and print API error details.
