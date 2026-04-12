# Multi-File Docker Compose Example

This example demonstrates the **recommended pattern** for deploying apps to Atmosphere using multiple Docker Compose files for different environments.

## Overview

This setup uses three compose files:
- `docker-compose.yml` - Base configuration (shared by all environments)
- `docker-compose.override.yml` - Local development (auto-loaded)
- `docker-compose.prod.yml` - Atmosphere/production deployment

## File Structure

```
multi-file-compose-app/
├── docker-compose.yml          # Base configuration
├── docker-compose.override.yml # Local development
├── docker-compose.prod.yml     # Production (Atmosphere)
├── Dockerfile
├── .env                        # Local environment
├── .env.example                # Template for production
├── src/
│   └── index.php
└── README.md
```

## Local Development

### Start Development Environment

```bash
# Uses docker-compose.yml + docker-compose.override.yml automatically
docker compose up -d

# Access app at http://localhost:8080
```

### Stop Development Environment

```bash
docker compose down
```

### Features (Local)
- ✅ Port 8080 exposed for direct access
- ✅ Source code mounted for live reloading
- ✅ Development environment variables
- ✅ No resource limits
- ✅ Debug logging enabled

## Production Deployment (Atmosphere)

### Prerequisites

1. **Generate SSH key** (without passphrase):
```bash
ssh-keygen -t ed25519 -C "atmosphere-deploy" -f ~/.ssh/atmosphere_deploy -N ""
```

2. **Add public key to GitHub**:
- Repository → Settings → Deploy keys
- Add content of `~/.ssh/atmosphere_deploy.pub`

3. **Push code to GitHub**:
```bash
git init
git add .
git commit -m "Initial commit"
git remote add origin git@github.com:yourusername/your-repo.git
git push -u origin main
```

### Deploy to Atmosphere

```bash
curl -X POST http://your-atmosphere-server:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "multi-file-app",
    "deployment_type": "github",
    "build_type": "compose",
    "compose_path": "docker-compose.prod.yml",
    "github_repo": "git@github.com:yourusername/your-repo.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/atmosphere_deploy)"'",
    "domain": "app.yourdomain.com",
    "env_vars": {
      "APP_ENV": "production",
      "SECRET_KEY": "your-production-secret"
    }
  }'

# Trigger deployment
curl -X POST http://your-atmosphere-server:3000/api/v1/apps/multi-file-app/deploy
```

### What Happens

1. Atmosphere clones your repository
2. Detects `docker-compose.prod.yml` from `compose_path`
3. Automatically uses **both** files:
   - `-f docker-compose.yml` (base)
   - `-f docker-compose.prod.yml` (override)
4. Builds and starts containers
5. Traefik routes HTTPS traffic to your app

### Features (Production)
- ✅ No exposed ports (Traefik handles routing)
- ✅ Automatic HTTPS with Let's Encrypt
- ✅ Resource limits (CPU/memory)
- ✅ Production environment variables
- ✅ Health checks
- ✅ Auto-restart on failure

## Environment Variables

### Automatically Injected by Atmosphere

These variables are available in production:

- `ATMOSPHERE_APP` - App name ("multi-file-app")
- `DOMAIN` - Your domain ("app.yourdomain.com")
- `TRAEFIK_NETWORK` - Traefik network ("traefik")

### User-Defined Variables

Set via `env_vars` in API request:

- `APP_ENV` - Application environment
- `SECRET_KEY` - Application secret key

## Testing Production Config Locally

To test the production configuration on your local machine:

```bash
# Create Traefik network (if it doesn't exist)
docker network create traefik

# View merged configuration
docker compose -f docker-compose.yml -f docker-compose.prod.yml config

# Start with production config (optional)
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Clean up
docker compose -f docker-compose.yml -f docker-compose.prod.yml down
```

## Troubleshooting

### Port Already Allocated

If port 8080 is in use, change it in `.env`:
```bash
WEB_PORT=8081
```

### Code Changes Not Reflecting

Ensure you're running the **override file**:
```bash
docker compose down
docker compose up -d  # Automatically loads override
```

### Can't Access Production App

1. Check DNS points to server IP:
```bash
nslookup app.yourdomain.com
```

2. Verify Traefik is running:
```bash
docker ps | grep traefik
```

3. Check deployment logs:
```bash
curl -s http://your-atmosphere-server:3000/api/v1/apps/multi-file-app/logs | jq -r '.[0].log'
```

4. Verify SSL certificate:
```bash
curl -I https://app.yourdomain.com
```

## Key Takeaways

1. **Three files, three purposes:**
   - `docker-compose.yml` - Shared base config
   - `docker-compose.override.yml` - Local dev only
   - `docker-compose.prod.yml` - Atmosphere/production

2. **Atmosphere field name:** Use `compose_path` (not `compose_file`)

3. **Auto-detection:** Atmosphere automatically merges base + prod files

4. **SSH keys:** Must be passphrase-free for automated deployment

5. **CPU limits:** Adjust based on server (0.9 max for 1-CPU servers)

6. **Traefik email:** Must be valid (not @example.com)

## Next Steps

- Read [DOCKER_COMPOSE_CONVERSION_GUIDE.md](../../docs/DOCKER_COMPOSE_CONVERSION_GUIDE.md)
- See [DEPLOYMENT_GUIDE.md](../../docs/DEPLOYMENT_GUIDE.md)
- Check [Traefik Documentation](https://doc.traefik.io/traefik/)
