# atmosphere

A lightweight, self-hosted deployment platform for Docker-based applications. Inspired by Coolify/Dokploy but simpler and focused only on Dockerfile and Docker Compose deployments.

## Features

- 🚀 Deploy apps from GitHub repositories using SSH deployment keys
- 📦 Deploy apps by uploading files manually
- 🔄 Automatic reverse proxy with Traefik
- 🔒 HTTPS support with Let's Encrypt
- 🐳 Support for Dockerfile and Docker Compose based apps
- 🔧 Simple REST API for management
- 📊 Deployment logs and status tracking

## Tech Stack

**Backend**: Go (chosen for reliability, low memory usage, single binary deployment)

**Database**: SQLite (easy to swap for PostgreSQL/MySQL later)

**Reverse Proxy**: Traefik (in Docker container)

**Runtime**: Docker Engine

## Installation

### Prerequisites

- Ubuntu 24.04 LTS (recommended)
- Root or sudo access
- Domain name pointing to your server (optional, for HTTPS)

### Quick Install

```bash
# Download and run installer
curl -fsSL https://raw.githubusercontent.com/kkassovic/atmosphere/main/install/install.sh -o install.sh
chmod +x install.sh
sudo ./install.sh
```

Or clone the repository and run:

```bash
git clone https://github.com/kkassovic/atmosphere.git
cd atmosphere
chmod +x install/install.sh
chmod +x install/install.sh
sudo ./install/install.sh
```

The installer will:
- Install Docker Engine and required tools
- Set up Traefik reverse proxy
- Create necessary directories and networks
- Install atmosphere as a systemd service
- Configure firewall rules

### Post-Installation

After installation, the atmosphere service will be running on port 3000.

```bash
# Check service status
sudo systemctl status atmosphere

# View logs
sudo journalctl -u atmosphere -f
```

## Configuration

Copy the example environment file and customize:

```bash
cd /opt/atmosphere
sudo cp .env.example .env
sudo nano .env
```

Key configuration options:
- `PORT`: API server port (default: 3000)
- `DATABASE_PATH`: SQLite database path
- `WORKSPACES_DIR`: Where app files are stored
- `KEYS_DIR`: Where deployment keys are stored
- `DOCKER_NETWORK`: Docker network for apps (default: atmosphere)
- `TRAEFIK_NETWORK`: Traefik network (default: traefik)
- `DOMAIN`: Your primary domain (optional)
- `LETSENCRYPT_EMAIL`: Email for Let's Encrypt certificates

## Usage

### API Endpoints

Base URL: `http://localhost:3000/api/v1`

#### Apps

**Create App (Manual Deployment)**
```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domain": "app.example.com"
  }'
```

**Create App (GitHub Deployment)**
```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "deployment_type": "github",
    "build_type": "compose",
    "domain": "app.example.com",
    "github_repo": "git@github.com:user/repo.git",
    "github_branch": "main",
    "deployment_key": "-----BEGIN OPENSSH PRIVATE KEY-----\n...\n-----END OPENSSH PRIVATE KEY-----"
  }'
```

**List Apps**
```bash
curl http://localhost:3000/api/v1/apps
```

**Get App Details**
```bash
curl http://localhost:3000/api/v1/apps/my-app
```

**Deploy App**
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

**Update App**
```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "newdomain.example.com",
    "env_vars": {
      "NODE_ENV": "production",
      "API_KEY": "secret"
    }
  }'
```

**Stop App**
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop
```

**Start App**
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/start
```

**Delete App**
```bash
curl -X DELETE http://localhost:3000/api/v1/apps/my-app
```

**Upload Files (Manual Deployment)**
```bash
# Upload a Dockerfile
curl -X POST http://localhost:3000/api/v1/apps/my-app/files \
  -F "path=Dockerfile" \
  -F "content=@./Dockerfile"

# Upload docker-compose.yml
curl -X POST http://localhost:3000/api/v1/apps/my-app/files \
  -F "path=docker-compose.yml" \
  -F "content=@./docker-compose.yml"
```

**View Deployment Logs**
```bash
curl http://localhost:3000/api/v1/apps/my-app/logs
```

### Manual Deployment Workflow

1. Create an app
2. Upload Dockerfile or docker-compose.yml
3. Upload any additional files needed (.env, source code, etc.)
4. Deploy the app

```bash
# 1. Create app
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{"name": "hello-world", "deployment_type": "manual", "build_type": "dockerfile", "domain": "hello.example.com"}'

# 2. Upload Dockerfile
curl -X POST http://localhost:3000/api/v1/apps/hello-world/files \
  -F "path=Dockerfile" \
  -F "content=FROM nginx:alpine
COPY index.html /usr/share/nginx/html/
EXPOSE 80"

# 3. Upload index.html
curl -X POST http://localhost:3000/api/v1/apps/hello-world/files \
  -F "path=index.html" \
  -F "content=<h1>hello from atmosphere!</h1>"

# 4. Deploy
curl -X POST http://localhost:3000/api/v1/apps/hello-world/deploy
```

### GitHub Deployment Workflow

1. Generate SSH deployment key on GitHub
2. Create app with repository details and deployment key
3. Deploy (app will be cloned and built)

```bash
# Create and deploy GitHub app
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-github-app",
    "deployment_type": "github",
    "build_type": "compose",
    "github_repo": "git@github.com:myorg/myrepo.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/deploy_key)"'",
    "domain": "myapp.example.com",
    "env_vars": {
      "DATABASE_URL": "postgresql://..."
    }
  }'

# Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-github-app/deploy
```

## How It Works

### Deployment Flow

1. **Preparation**
   - Create app workspace directory
   - For GitHub: clone/pull repository using deployment key
   - For Manual: files already uploaded to workspace
   - Detect build type (Dockerfile or docker-compose.yml)

2. **Build**
   - For Dockerfile: `docker build -t atmosphere-{app-name} .`
   - For Compose: `docker compose build`
   - Inject environment variables

3. **Deploy**
   - Stop old containers if running
   - Apply Traefik labels for routing
   - Start new containers
   - For Dockerfile: `docker run` with appropriate labels
   - For Compose: `docker compose up -d`

4. **Routing**
   - Traefik automatically detects containers via Docker provider
   - Routes traffic based on Host rules
   - Handles Let's Encrypt certificates automatically

### Traefik Integration

Apps are automatically routed through Traefik using Docker labels:

```yaml
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.{app}.rule=Host(`{domain}`)"
  - "traefik.http.routers.{app}.entrypoints=websecure"
  - "traefik.http.routers.{app}.tls.certresolver=letsencrypt"
  - "traefik.http.services.{app}.loadbalancer.server.port={port}"
```

Traefik runs in its own container and listens on:
- Port 80 (HTTP) - redirects to HTTPS
- Port 443 (HTTPS) - serves applications

### Directory Structure

After installation:

```
/opt/atmosphere/
├── atmosphere          # Binary
├── .env                # Configuration
├── atmosphere.db       # SQLite database
├── workspaces/         # App files
│   └── {app-name}/     # Per-app directory
├── keys/               # SSH deployment keys
│   └── {app-name}.key
└── logs/               # Deployment logs
    └── {app-name}/

/opt/traefik/
├── traefik.yml         # Static config
├── acme.json           # Let's Encrypt certificates
└── docker-compose.yml
```

## Security Considerations

- Deployment keys stored with 0600 permissions
- Secrets not exposed in logs
- Input validation for app names, domains, paths
- Path traversal prevention
- Designed for single-server, trusted admin use

## Development

### Building from Source

```bash
cd backend
go mod download
go build -o ../atmosphere ./cmd/atmosphere
```

### Running Locally

```bash
# Set up environment
cp .env.example .env

# Run
./atmosphere
```

### Testing

```bash
cd backend
go test ./...
```

## Roadmap

- [ ] Web GUI frontend
- [ ] Multiple deployment targets
- [ ] Advanced monitoring
- [ ] Backup/restore functionality
- [ ] PostgreSQL/MySQL support
- [ ] User authentication
- [ ] Webhook support

## License

MIT

## Contributing

Contributions welcome! Please open an issue or PR.

## Support

For issues and questions, please use GitHub Issues.
