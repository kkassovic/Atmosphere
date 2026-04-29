# atmosphere - Complete Workflow Guide

This document explains how everything works together in atmosphere.

## Table of Contents

1. [System Overview](#system-overview)
2. [Installation Flow](#installation-flow)
3. [GitHub Deployment Flow](#github-deployment-flow)
4. [Manual Deployment Flow](#manual-deployment-flow)
5. [Traefik Routing Flow](#traefik-routing-flow)
6. [Request Flow](#request-flow)

---

## System Overview

atmosphere consists of four main components:

1. **atmosphere Service** (Go backend)
   - REST API
   - Deployment orchestration
   - Database management

2. **Traefik** (Reverse proxy)
   - Routes incoming requests
   - Handles SSL/TLS
   - Service discovery

3. **Docker Engine**
   - Runs application containers
   - Provides isolation

4. **SQLite Database**
   - Stores app metadata
   - Tracks deployment history

---

## Installation Flow

### What install.sh Does

```
┌─────────────────────────────────────────────┐
│ 1. System Preparation                       │
│    - Update package lists                   │
│    - Install prerequisites (curl, git, jq)  │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 2. Docker Installation                      │
│    - Add Docker repository                  │
│    - Install Docker Engine                  │
│    - Install Docker Compose plugin          │
│    - Enable and start Docker service        │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 3. Network Creation                         │
│    - Create 'atmosphere' network            │
│    - Create 'traefik' network               │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 4. Directory Setup                          │
│    - /opt/atmosphere/                       │
│      ├── workspaces/                        │
│      ├── keys/ (chmod 700)                  │
│      └── logs/                              │
│    - /opt/traefik/                          │
│      └── acme/                              │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 5. Configuration                            │
│    - Create .env file                       │
│    - Create traefik.yml                     │
│    - Create traefik docker-compose.yml      │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 6. Traefik Deployment                       │
│    - Start Traefik container                │
│    - Bind ports 80, 443                     │
│    - Mount Docker socket                    │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 7. atmosphere Build & Installation          │
│    - Download Go (if needed)                │
│    - Build atmosphere binary                │
│    - Copy to /opt/atmosphere/               │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 8. Systemd Service                          │
│    - Create atmosphere.service              │
│    - Enable service                         │
│    - Start service                          │
└────────────────┬────────────────────────────┘
                 │
                 ▼
┌─────────────────────────────────────────────┐
│ 9. Firewall Configuration                   │
│    - Allow SSH (22)                         │
│    - Allow HTTP (80)                        │
│    - Allow HTTPS (443)                      │
│    - Allow API (3000)                       │
└────────────────┬────────────────────────────┘
                 │
                 ▼
          ✅ Installation Complete
```

### Post-Install State

After installation:
- Traefik is running on ports 80, 443
- atmosphere API is available on port 3000
- Ready to accept deployments
- systemd will auto-start on boot

---

## GitHub Deployment Flow

### Complete Deployment Sequence

```
User creates app via API
         │
         ▼
┌────────────────────────────────────────────┐
│ API Handler: CreateApp                     │
│ - Validates request                        │
│ - Checks for duplicates                    │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ AppService: CreateApp                      │
│ - Creates workspace directory:             │
│   /opt/atmosphere/workspaces/{app-name}/   │
│ - Saves deployment key:                    │
│   /opt/atmosphere/keys/{app-name}.key      │
│ - Sets permissions to 0600                 │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ Repository: Create                         │
│ - Inserts app into SQLite                  │
│ - Status: "stopped"                        │
└────────┬───────────────────────────────────┘
         │
         ▼
     App Created ✅
         │
         │ User calls /deploy
         ▼
┌────────────────────────────────────────────┐
│ API Handler: DeployApp                     │
│ - Gets app from database                   │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ AppService: DeployApp                      │
│ - Updates status to "building"             │
│ - Creates deployment log entry             │
│ - Spawns background goroutine              │
└────────┬───────────────────────────────────┘
         │
         ▼ (Background goroutine)
┌────────────────────────────────────────────┐
│ DeploymentService: Deploy                  │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ STEP 1: Prepare Repository                 │
│                                             │
│ Check if .git exists in workspace          │
│   ├─ YES → git pull origin {branch}        │
│   └─ NO  → git clone {repo} {workspace}    │
│                                             │
│ Uses GIT_SSH_COMMAND with deployment key   │
│ Command: ssh -i {key} -o StrictHost...     │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ STEP 2: Detect Build Type                  │
│                                             │
│ Look for files in workspace:               │
│   - docker-compose.yml                     │
│   - docker-compose.yaml                    │
│   - compose.yml                            │
│   - compose.yaml                           │
│   - Dockerfile                             │
│                                             │
│ User's build_type determines strategy      │
└────────┬───────────────────────────────────┘
         │
         ├─── build_type: "dockerfile" ────┐
         │                                  │
         │                                  ▼
         │              ┌────────────────────────────────┐
         │              │ Docker Build Process           │
         │              │                                │
         │              │ 1. Create tar of workspace     │
         │              │ 2. docker build -t             │
         │              │    atmosphere-{app}:latest     │
         │              │ 3. Capture build logs          │
         │              └──────────┬─────────────────────┘
         │                         │
         │                         ▼
         │              ┌────────────────────────────────┐
         │              │ Container Creation             │
         │              │                                │
         │              │ 1. Generate Traefik labels     │
         │              │ 2. Create container with:      │
         │              │    - Image: atmosphere-{app}   │
         │              │    - Networks: atmosphere,     │
         │              │      traefik                   │
         │              │    - Env vars from app config  │
         │              │    - Labels for routing        │
         │              │ 3. Start container             │
         │              └──────────┬─────────────────────┘
         │                         │
         └─── build_type: "compose" ───────┘
                                   │
                                   ▼
         ┌────────────────────────────────────────┐
         │ Docker Compose Process                 │
         │                                         │
         │ 1. Create .env.atmosphere with vars    │
         │ 2. docker compose -p                   │
         │    atmosphere-{app} build              │
         │ 3. docker compose -p                   │
         │    atmosphere-{app} up -d              │
         │ 4. Env vars injected:                  │
         │    - ATMOSPHERE_APP={app-name}         │
         │    - TRAEFIK_NETWORK=traefik           │
         └──────────┬─────────────────────────────┘
                    │
                    ▼
         ┌────────────────────────────────────────┐
         │ STEP 3: Stop Old Containers            │
         │                                         │
         │ 1. Find containers with label:         │
         │    atmosphere.app={app-name}           │
         │ 2. Stop each container                 │
         │ 3. Remove each container               │
         └──────────┬─────────────────────────────┘
                    │
                    ▼
         ┌────────────────────────────────────────┐
         │ STEP 4: Update Database                │
         │                                         │
         │ - Update deployment log:               │
         │   - status: "success" or "failed"      │
         │   - log: complete output               │
         │   - ended_at: timestamp                │
         │                                         │
         │ - Update app:                          │
         │   - status: "running" or "failed"      │
         │   - last_deployed_at: timestamp        │
         └──────────┬─────────────────────────────┘
                    │
                    ▼
             Deployment Complete ✅
                    │
                    ▼
         ┌────────────────────────────────────────┐
         │ Traefik Auto-Discovery                 │
         │                                         │
         │ 1. Traefik detects new container       │
         │    via Docker socket                   │
         │ 2. Reads labels from container         │
         │ 3. Creates HTTP router                 │
         │ 4. Configures service backend          │
         │ 5. Requests SSL cert (if domain)       │
         │ 6. Begins routing traffic              │
         └────────────────────────────────────────┘
```

---

## Manual Deployment Flow

### Upload and Deploy Sequence

```
User creates app
         │
         ▼
┌────────────────────────────────────────────┐
│ AppService: CreateApp                      │
│ - deployment_type: "manual"                │
│ - Creates empty workspace                  │
└────────┬───────────────────────────────────┘
         │
         ▼
     App Created ✅
         │
         │ User uploads files
         ▼
┌────────────────────────────────────────────┐
│ API Handler: UploadFile                    │
│ - Receives multipart/form-data             │
│ - Fields: path, content                    │
└────────┬───────────────────────────────────┘
         │
         ▼
┌────────────────────────────────────────────┐
│ AppService: UploadFile                     │
│                                             │
│ 1. Validate app exists                     │
│ 2. Validate deployment_type == "manual"    │
│ 3. Validate file path:                     │
│    - No absolute paths                     │
│    - No path traversal (..)                │
│    - Must be within workspace              │
│ 4. Create parent directories               │
│ 5. Write file to:                          │
│    /opt/atmosphere/workspaces/             │
│    {app-name}/{path}                       │
└────────┬───────────────────────────────────┘
         │
         ▼
     File Uploaded ✅
         │
         │ (Repeat for all files)
         │
         │ User calls /deploy
         ▼
┌────────────────────────────────────────────┐
│ DeploymentService: Deploy                  │
│                                             │
│ 1. Uses existing workspace files           │
│ 2. No git clone/pull                       │
│ 3. Build and deploy same as GitHub flow    │
└────────────────────────────────────────────┘
```

### File Upload Example

```bash
# Upload Dockerfile
POST /api/v1/apps/myapp/files
Content-Type: multipart/form-data

path=Dockerfile
content=FROM nginx:alpine
        COPY index.html /usr/share/nginx/html
        EXPOSE 80

# Saved to:
/opt/atmosphere/workspaces/myapp/Dockerfile

# Upload another file
POST /api/v1/apps/myapp/files

path=index.html
content=<h1>Hello World</h1>

# Saved to:
/opt/atmosphere/workspaces/myapp/index.html
```

---

## Traefik Routing Flow

### How Traffic Reaches Your App

```
┌─────────────────────────────────────────────┐
│ 1. User Request                             │
│    https://myapp.example.com                │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 2. DNS Resolution                           │
│    myapp.example.com → YOUR_SERVER_IP       │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 3. Arrives at Server                        │
│    Port 443 (HTTPS)                         │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 4. Traefik Container                        │
│    - Listening on ports 80, 443             │
│    - TLS termination                        │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 5. Router Matching                          │
│                                             │
│ Traefik checks all routers:                │
│                                             │
│ Router: myapp                               │
│   Rule: Host(`myapp.example.com`)          │
│   √ MATCH!                                  │
│                                             │
│ Router: otherapp                            │
│   Rule: Host(`other.example.com`)          │
│   ✗ No match                                │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 6. Service Resolution                       │
│                                             │
│ Router "myapp" → Service "myapp"           │
│ Service port: 3000                         │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 7. Forward to Container                     │
│                                             │
│ Target: atmosphere-myapp container          │
│ Port: 3000                                  │
│ Network: traefik                            │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 8. Application Container                    │
│    - Receives request                       │
│    - Processes                              │
│    - Returns response                       │
└────────────┬────────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────────┐
│ 9. Response Path                            │
│    Container → Traefik → User               │
└─────────────────────────────────────────────┘
```

### How Labels Create Routes

When atmosphere deploys a container:

```go
labels := map[string]string{
    "traefik.enable": "true",
    "traefik.http.routers.myapp.rule": "Host(`myapp.example.com`)",
    "traefik.http.routers.myapp.entrypoints": "websecure",
    "traefik.http.routers.myapp.tls": "true",
    "traefik.http.routers.myapp.tls.certresolver": "letsencrypt",
    "traefik.http.services.myapp.loadbalancer.server.port": "3000",
    "atmosphere.app": "myapp",
}
```

Traefik reads these labels and:
1. Creates router "myapp"
2. Rule: Host matches `myapp.example.com`
3. Entry point: `websecure` (port 443)
4. TLS enabled with Let's Encrypt
5. Forwards to container port 3000

---

## Request Flow

### API Request to Deploy

```
curl -X POST http://localhost:3000/api/v1/apps/myapp/deploy
                    │
                    ▼
        ┌───────────────────────┐
        │ Chi Router            │
        │ POST /api/v1/apps/    │
        │ {name}/deploy         │
        └──────┬────────────────┘
               │
               ▼
        ┌───────────────────────┐
        │ Middleware Stack      │
        │ - RequestID           │
        │ - Logger              │
        │ - Recoverer           │
        │ - Timeout (60s)       │
        └──────┬────────────────┘
               │
               ▼
        ┌───────────────────────┐
        │ Handler.DeployApp     │
        │ - Extract name param  │
        └──────┬────────────────┘
               │
               ▼
        ┌───────────────────────┐
        │ AppService.DeployApp  │
        │ - Get app from DB     │
        │ - Validate state      │
        │ - Create deploy log   │
        │ - Spawn goroutine     │
        └──────┬────────────────┘
               │
               ▼
        ┌───────────────────────┐
        │ HTTP Response 202     │
        │ Accepted              │
        │ {                     │
        │   "message": "...",   │
        │   "deployment_log":   │
        │   {...}               │
        │ }                     │
        └───────────────────────┘
```

### Background Deployment

```
Goroutine spawned
        │
        ▼
┌────────────────────────────────┐
│ DeploymentService.Deploy       │
│ - Clone/pull repo              │
│ - Build Docker image           │
│ - Create containers            │
│ - Start containers             │
│ - Capture all logs             │
└────────┬───────────────────────┘
         │
         ▼
┌────────────────────────────────┐
│ Update Database                │
│ - Deployment log status        │
│ - Deployment log output        │
│ - App status                   │
│ - Last deployed timestamp      │
└────────────────────────────────┘
```

### User Checking Status

```
curl http://localhost:3000/api/v1/apps/myapp
        │
        ▼
┌────────────────────────────────┐
│ Handler.GetApp                 │
└────────┬───────────────────────┘
         │
         ▼
┌────────────────────────────────┐
│ AppService.GetApp              │
│ - Query database               │
└────────┬───────────────────────┘
         │
         ▼
┌────────────────────────────────┐
│ Repository.GetByName           │
│ - SELECT * FROM apps WHERE..   │
└────────┬───────────────────────┘
         │
         ▼
┌────────────────────────────────┐
│ Response                       │
│ {                              │
│   "name": "myapp",             │
│   "status": "running",         │
│   "domains": ["..."],          │
│   "last_deployed_at": "..."    │
│ }                              │
└────────────────────────────────┘
```

---

## Summary

### Key Concepts

1. **Async Deployment**: Deployments run in background, API returns immediately
2. **Label-Based Routing**: Traefik auto-discovers containers via labels
3. **Workspace Isolation**: Each app has its own directory
4. **Key Security**: SSH keys stored with strict permissions
5. **Database Tracking**: All state persisted in SQLite

### Complete Lifecycle

```
Create App → Upload Files/Clone Repo → Deploy → 
  Build → Start Containers → Traefik Routes → 
    Live Application → Monitor Logs → Redeploy/Update
```

### Important Paths

- Apps metadata: SQLite `/opt/atmosphere/atmosphere.db`
- App files: `/opt/atmosphere/workspaces/{app-name}/`
- Deployment keys: `/opt/atmosphere/keys/{app-name}.key`
- Traefik certs: `/opt/traefik/acme/acme.json`
- Service logs: `journalctl -u atmosphere`

---

This completes the workflow documentation. For more details on specific components, see:
- [ARCHITECTURE.md](ARCHITECTURE.md) - Technical architecture
- [DEPLOYMENT_GUIDE.md](DEPLOYMENT_GUIDE.md) - Deployment instructions
- [README.md](../README.md) - General usage
