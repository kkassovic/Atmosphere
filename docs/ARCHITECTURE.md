# Architecture Documentation

## Overview

Atmosphere is a lightweight deployment platform built with Go, designed to deploy Docker-based applications on a single Linux server with automatic reverse proxy routing via Traefik.

## System Components

```
┌─────────────────────────────────────────────────────────────┐
│                        Internet                              │
└────────────────────────┬────────────────────────────────────┘
                         │
                         │ HTTPS (443) / HTTP (80)
                         ▼
              ┌──────────────────────┐
              │   Traefik Proxy      │
              │   (Docker Container) │
              └──────────┬───────────┘
                         │
                         │ Routes based on domain
                         │
         ┌───────────────┼───────────────┐
         │               │               │
         ▼               ▼               ▼
    ┌────────┐     ┌────────┐     ┌────────┐
    │ App 1  │     │ App 2  │     │ App 3  │
    │Container     │Container     │Container
    └────────┘     └────────┘     └────────┘
         ▲               ▲               ▲
         │               │               │
         └───────────────┴───────────────┘
                         │
                         │ Manages containers
                         │
              ┌──────────────────────┐
              │  Atmosphere Service  │
              │  (Go Binary)         │
              │  - API Server        │
              │  - Deployment Logic  │
              │  - Docker Integration│
              └──────────┬───────────┘
                         │
                         │ Reads/Writes
                         ▼
              ┌──────────────────────┐
              │  SQLite Database     │
              │  - Apps              │
              │  - Deployment Logs   │
              └──────────────────────┘
```

## Technology Stack

### Backend: Go
- **Why Go**: Single binary deployment, low memory footprint, excellent concurrency, great Docker SDK
- **Framework**: Chi router for HTTP routing
- **Database Driver**: mattn/go-sqlite3
- **Docker SDK**: Official Docker SDK for Go

### Database: SQLite
- **Why SQLite**: Simple, serverless, zero configuration
- **Future**: Can be swapped for PostgreSQL/MySQL via repository pattern
- **Schema**: Apps, deployment logs with foreign keys

### Reverse Proxy: Traefik
- **Why Traefik**: Automatic service discovery, built-in Let's Encrypt, Docker-native
- **Version**: Traefik v2.11
- **Features Used**: Docker provider, HTTP challenge, automatic HTTPS

### Container Runtime: Docker
- **Requirements**: Docker Engine 24.0+, Docker Compose plugin
- **Networks**: `atmosphere` (apps), `traefik` (proxy)

## Code Architecture

### Layers

```
cmd/atmosphere/           - Entry point, server initialization
internal/
  ├── config/            - Configuration loading and validation
  ├── models/            - Data models (App, DeploymentLog, etc.)
  ├── database/          - Database initialization and migrations
  ├── repository/        - Data access layer (CRUD operations)
  ├── services/          - Business logic layer
  │   ├── app_service.go         - App lifecycle management
  │   ├── deployment_service.go  - Deployment orchestration
  │   └── docker_service.go      - Docker API wrapper
  └── api/               - HTTP layer
      ├── routes.go      - Route definitions
      └── handlers.go    - HTTP handlers
```

### Design Patterns

**Repository Pattern**: Data access is abstracted behind repository interfaces, making it easy to swap SQLite for another database.

**Service Layer**: Business logic is separated from HTTP handlers, making it testable and reusable.

**Dependency Injection**: Services receive their dependencies via constructors.

## Data Flow

### Creating an App

```
1. HTTP POST /api/v1/apps
   ↓
2. Handler validates and parses request
   ↓
3. AppService.CreateApp()
   - Validates app name, deployment type
   - Creates workspace directory
   - Saves deployment key (if GitHub)
   - Calls repository to persist
   ↓
4. Repository.Create()
   - Inserts into SQLite
   ↓
5. Returns created app
```

### Deploying an App

```
1. HTTP POST /api/v1/apps/{name}/deploy
   ↓
2. Handler calls AppService.DeployApp()
   ↓
3. AppService creates deployment log, sets status to "building"
   ↓
4. Spawns goroutine for async deployment
   ↓
5. DeploymentService.Deploy()
   - For GitHub: Clone/pull repository
   - For Manual: Use uploaded files
   - Detect build type (Dockerfile vs Compose)
   - Build Docker image or run compose build
   - Stop old containers
   - Start new containers with Traefik labels
   - Update deployment log
   ↓
6. Background goroutine updates status to "running" or "failed"
```

## Security Considerations

### Deployment Keys
- Stored in `/opt/atmosphere/keys/` with 0600 permissions
- Only accessible by root (Atmosphere runs as root for Docker access)
- Never logged or returned in API responses

### Input Validation
- App names: lowercase alphanumeric + hyphens, max 32 chars
- Domains: standard domain format validation
- File paths: prevented path traversal (no `..`, absolute paths rejected)

### Network Isolation
- Apps run in dedicated `atmosphere` network
- Only containers explicitly connected to `traefik` network are routable
- Traefik provides SSL termination

### Secrets Management
- Environment variables stored encrypted in SQLite (can be enhanced)
- Deployment keys have strict file permissions
- No secrets in logs or error messages

## Deployment Flow Detail

### GitHub Deployment

```
1. User creates app with:
   - GitHub repo URL
   - Branch name
   - Deployment key (SSH private key)
   
2. Atmosphere saves deployment key to:
   /opt/atmosphere/keys/{app-name}.key (0600)

3. On deploy:
   a. Check if repo exists locally
   b. If yes: git pull with deployment key
   c. If no: git clone with deployment key
   d. Detect Dockerfile or docker-compose.yml
   e. Build image(s)
   f. Create containers with labels
   g. Start containers

4. Traefik detects new containers via Docker socket
5. Routes traffic based on Host() rule
```

### Manual Deployment

```
1. User creates app (deployment_type: "manual")
2. User uploads files via API:
   POST /api/v1/apps/{name}/files
   - Dockerfile or docker-compose.yml
   - Application code
   - .env files
   
3. Files saved to:
   /opt/atmosphere/workspaces/{app-name}/

4. On deploy:
   a. Read files from workspace
   b. Build and deploy same as GitHub flow
```

## Traefik Integration

### Label-Based Routing

Apps are automatically routed by attaching Docker labels:

```yaml
labels:
  traefik.enable: "true"
  traefik.http.routers.{app}.rule: "Host(`{domain}`)"
  traefik.http.routers.{app}.entrypoints: "websecure"
  traefik.http.routers.{app}.tls: "true"
  traefik.http.routers.{app}.tls.certresolver: "letsencrypt"
  traefik.http.services.{app}.loadbalancer.server.port: "{port}"
  atmosphere.app: "{app-name}"
```

### Certificate Management

Traefik automatically:
1. Listens for HTTP-01 challenges on port 80
2. Obtains certificates from Let's Encrypt
3. Stores certificates in `/opt/traefik/acme/acme.json`
4. Renews certificates before expiry
5. Redirects HTTP → HTTPS

## Scalability & Future Enhancements

### Current Limitations
- Single server only
- No horizontal scaling
- No multi-tenancy / user auth
- Basic monitoring

### Future Enhancements
- **Web UI**: React/Vue frontend
- **Multi-server**: Deploy across multiple nodes
- **User Management**: Authentication and authorization
- **Monitoring**: Prometheus + Grafana integration
- **Backups**: Automated backup/restore
- **Webhooks**: GitHub webhook support for auto-deploy
- **Database**: PostgreSQL support for larger deployments
- **Advanced Networking**: Custom networks, service mesh

## File System Layout

```
/opt/atmosphere/
├── atmosphere              # Go binary
├── .env                    # Configuration
├── atmosphere.db           # SQLite database
├── workspaces/             # App files
│   ├── app-1/
│   │   ├── .git/          # For GitHub deployments
│   │   ├── Dockerfile
│   │   └── ...
│   └── app-2/
│       ├── docker-compose.yml
│       └── ...
├── keys/                   # SSH deployment keys (0700)
│   ├── app-1.key          # (0600)
│   └── app-2.key
└── logs/                   # Deployment logs (future use)

/opt/traefik/
├── traefik.yml             # Static config
├── docker-compose.yml
└── acme/
    └── acme.json          # Let's Encrypt certs (0600)
```

## Docker Networks

### atmosphere
- Purpose: Default network for deployed apps
- Type: Bridge network
- Use: Apps that don't need external access

### traefik
- Purpose: Network for Traefik-routed apps
- Type: Bridge network
- Use: All publicly accessible apps must be on this network

## Monitoring & Logs

### Atmosphere Logs
```bash
journalctl -u atmosphere -f
```

### Traefik Logs
```bash
cd /opt/traefik
docker compose logs -f
```

### App Container Logs
```bash
docker logs -f atmosphere-{app-name}
```

### Deployment Logs
Stored in database, accessible via API:
```bash
curl http://localhost:3000/api/v1/apps/{name}/logs
```

## Error Handling

### Build Failures
- Status set to "failed"
- Logs captured and stored
- Old containers remain running

### Deployment Failures
- Rollback not automatic (future enhancement)
- Manual intervention required
- Logs show failure reason

### Runtime Failures
- Containers restart automatically (unless-stopped policy)
- Check container logs for errors
- Update app config and redeploy
