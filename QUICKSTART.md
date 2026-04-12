# Quick Start Guide

Get atmosphere up and running in 5 minutes.

## Prerequisites

- Ubuntu 24.04 LTS server
- Root or sudo access
- Domain name (optional, for HTTPS)

## Installation

### 1. Clone or Download

```bash
git clone https://github.com/kkassovic/atmosphere.git
cd atmosphere
```

Or download:
```bash
wget https://github.com/kkassovic/atmosphere/archive/main.zip
unzip main.zip
cd atmosphere-main
```

### 2. Run Installer

```bash
chmod +x install/install.sh
sudo ./install/install.sh
```

This will:
- Install Docker and required tools
- Set up Traefik reverse proxy
- Build and install atmosphere
- Create systemd service
- Configure firewall

⏱️ Takes about 5-10 minutes depending on your connection.

### 3. Configure

Edit the configuration file:

```bash
sudo nano /opt/atmosphere/.env
```

**Important**: Set your email for Let's Encrypt:
```bash
LETSENCRYPT_EMAIL=your-email@example.com
```

**Also configure Traefik's email** (required for Let's Encrypt):
```bash
sudo nano /opt/traefik/traefik.yml
# Change email from example.com to your real email address
```

Restart both services:
```bash
sudo systemctl restart atmosphere
cd /opt/traefik && docker compose restart
```

### 4. Verify Installation

Check atmosphere is running:
```bash
sudo systemctl status atmosphere
```

Check Traefik is running:
```bash
cd /opt/traefik
docker compose ps
```

Test the API:
```bash
curl http://localhost:3000/health
```

Should return: `{"status":"ok"}`

---

## Deploy Your First App

### Example 1: Deploy a Static Website (Manual)

```bash
# Create the app
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-site",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domain": "mysite.example.com",
    "port": 80
  }'

# Upload Dockerfile
curl -X POST http://localhost:3000/api/v1/apps/my-site/files \
  -F "path=Dockerfile" \
  -F 'content=FROM nginx:alpine
COPY index.html /usr/share/nginx/html/
EXPOSE 80'

# Upload HTML
curl -X POST http://localhost:3000/api/v1/apps/my-site/files \
  -F "path=index.html" \
  -F 'content=<h1>hello from atmosphere!</h1>'

# Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-site/deploy

# Check status
curl http://localhost:3000/api/v1/apps/my-site
```

Visit `https://mysite.example.com` (make sure DNS points to your server!)

### Example 2: Deploy from GitHub

First, generate an SSH key:
```bash
ssh-keygen -t ed25519 -C "atmosphere-deploy" -f ~/.ssh/atmosphere_key
```

Add the **public key** (`~/.ssh/atmosphere_key.pub`) to your GitHub repository:
- Repo → Settings → Deploy keys → Add deploy key

Then deploy:

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "deployment_type": "github",
    "build_type": "dockerfile",
    "github_repo": "git@github.com:yourusername/yourrepo.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/atmosphere_key)"'",
    "domain": "app.example.com",
    "env_vars": {
      "NODE_ENV": "production"
    }
  }'

# Deploy
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy

# Watch logs
curl http://localhost:3000/api/v1/apps/my-app/logs
```

---

## Common Commands

### List All Apps
```bash
curl http://localhost:3000/api/v1/apps
```

### Get App Details
```bash
curl http://localhost:3000/api/v1/apps/my-app
```

### Redeploy (pull latest changes)
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Stop App
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop
```

### Start App
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/start
```

### Delete App
```bash
curl -X DELETE http://localhost:3000/api/v1/apps/my-app
```

### View Deployment Logs
```bash
curl http://localhost:3000/api/v1/apps/my-app/logs
```

---

## Troubleshooting

### Service Not Starting

```bash
# Check logs
sudo journalctl -u atmosphere -f

# Check if Docker is running
sudo systemctl status docker

# Restart service
sudo systemctl restart atmosphere
```

### App Won't Deploy

```bash
# View deployment logs
curl http://localhost:3000/api/v1/apps/my-app/logs

# Check container logs
docker ps
docker logs atmosphere-my-app
```

### Can't Access via Domain

**Check DNS:**
```bash
nslookup myapp.example.com
# Should point to your server IP
```

**Check Traefik:**
```bash
cd /opt/traefik
docker compose logs traefik | tail -50
```

**Check ports:**
```bash
sudo ufw status
# Should allow 80 and 443
```

**Check certificate:**
```bash
cd /opt/traefik
docker compose logs | grep acme
```

### SSL Certificate Issues

Let's Encrypt requires:
- Port 80 accessible (for HTTP challenge)
- Domain pointing to server
- Valid email in Traefik config

Check email is set:
```bash
grep LETSENCRYPT_EMAIL /opt/atmosphere/.env
```

---

## Next Steps

1. **Read the docs**: See [README.md](README.md) for full API documentation
2. **Deploy real apps**: Try the [examples](examples/) - especially the [multi-file compose pattern](examples/multi-file-compose-app/)
3. **Learn to update**: See [docs/UPDATING.md](docs/UPDATING.md) for keeping Atmosphere up to date
4. **Set up monitoring**: Check container logs regularly
5. **Configure backups**: Backup `/opt/atmosphere/` regularly
6. **Secure your server**: Configure firewall, SSH keys, fail2ban

---

## Getting Help

- **Documentation**: 
  - [Deployment Guide](docs/DEPLOYMENT_GUIDE.md) - Detailed deployment workflows
  - [Docker Compose Conversion](docs/DOCKER_COMPOSE_CONVERSION_GUIDE.md) - Convert existing apps
  - [Updating Guide](docs/UPDATING.md) - Keep Atmosphere up to date
  - [Architecture](docs/ARCHITECTURE.md) - System internals
- **Issues**: Check logs first, then open an issue on GitHub
- **Examples**: See [examples/](examples/) for working apps

---

## Quick Reference

### File Locations

```
/opt/atmosphere/           # Main directory
├── atmosphere             # Binary
├── .env                   # Config
├── atmosphere.db          # Database
├── workspaces/            # App files
├── keys/                  # SSH keys
└── logs/                  # Logs

/opt/traefik/              # Traefik
├── traefik.yml
├── docker-compose.yml
└── acme/acme.json        # SSL certificates
```

### Service Management

```bash
# Status
sudo systemctl status atmosphere

# Start
sudo systemctl start atmosphere

# Stop
sudo systemctl stop atmosphere

# Restart
sudo systemctl restart atmosphere

# Logs
sudo journalctl -u atmosphere -f
```

### Docker Commands

```bash
# List atmosphere containers
docker ps | grep atmosphere

# View container logs
docker logs -f atmosphere-myapp

# Execute command in container
docker exec -it atmosphere-myapp /bin/sh

# Clean up unused images
docker system prune -a
```

---

## Support

For detailed information:
- [Architecture](docs/ARCHITECTURE.md)
- [Deployment Guide](docs/DEPLOYMENT_GUIDE.md)
- [Workflow](docs/WORKFLOW.md)

Happy deploying! 🚀
