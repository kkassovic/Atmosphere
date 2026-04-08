# Deployment Guide

This guide explains how different deployment workflows work in atmosphere.

## Table of Contents

1. [GitHub Deployment with SSH Keys](#github-deployment-with-ssh-keys)
2. [Manual File Deployment](#manual-file-deployment)
3. [Docker Compose Deployment](#docker-compose-deployment)
4. [Dockerfile Deployment](#dockerfile-deployment)
5. [Environment Variables](#environment-variables)
6. [Domain Configuration](#domain-configuration)
7. [Troubleshooting](#troubleshooting)

---

## GitHub Deployment with SSH Keys

### Overview

GitHub deployment allows atmosphere to automatically clone and deploy applications from private or public GitHub repositories using SSH deployment keys.

### Prerequisites

1. A GitHub repository with your application
2. SSH deployment key (read-only access)

### Step 1: Generate SSH Deployment Key

On your local machine:

```bash
ssh-keygen -t ed25519 -C "atmosphere-deploy" -f ~/.ssh/atmosphere_deploy
```

This creates:
- `~/.ssh/atmosphere_deploy` - Private key (keep secret)
- `~/.ssh/atmosphere_deploy.pub` - Public key (add to GitHub)

### Step 2: Add Public Key to GitHub

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Deploy keys**
3. Click **Add deploy key**
4. Paste the contents of `atmosphere_deploy.pub`
5. Give it a name (e.g., "atmosphere Deployment")
6. **Do not** check "Allow write access"
7. Click **Add key**

### Step 3: Create App in atmosphere

**Using default docker-compose.yml or Dockerfile:**

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-github-app",
    "deployment_type": "github",
    "build_type": "compose",
    "github_repo": "git@github.com:username/repository.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/atmosphere_deploy)"'",
    "domain": "app.example.com",
    "env_vars": {
      "NODE_ENV": "production",
      "DATABASE_URL": "postgresql://..."
    }
  }'
```

**Using custom docker-compose file:**

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-github-app",
    "deployment_type": "github",
    "build_type": "compose",
    "compose_file": "docker-compose.prod.yml",
    "github_repo": "git@github.com:username/repository.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/atmosphere_deploy)"'",
    "domain": "app.example.com",
    "env_vars": {
      "NODE_ENV": "production",
      "DATABASE_URL": "postgresql://..."
    }
  }'
```

**Using custom Dockerfile:**

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-github-app",
    "deployment_type": "github",
    "build_type": "dockerfile",
    "dockerfile_path": "docker/Dockerfile.prod",
    "github_repo": "git@github.com:username/repository.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/atmosphere_deploy)"'",
    "domain": "app.example.com",
    "port": 3000,
    "env_vars": {
      "NODE_ENV": "production",
      "DATABASE_URL": "postgresql://..."
    }
  }'
```

**Important**: 
- Use the SSH URL format: `git@github.com:username/repo.git`
- Include the entire private key in `deployment_key`
- Specify the branch to deploy
- **Optional fields:**
  - `compose_file` - Path to custom docker-compose file (default: `docker-compose.yml`)
  - `dockerfile_path` - Path to custom Dockerfile (default: `Dockerfile`)
  - Paths are relative to repository root

### Step 4: Deploy

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-github-app/deploy
```

### What Happens

1. atmosphere saves the deployment key to `/opt/atmosphere/keys/my-github-app.key`
2. On deploy, it clones the repository (or pulls if already cloned)
3. Detects Dockerfile or docker-compose.yml
4. Builds the application
5. Starts containers with Traefik labels
6. Traefik routes traffic to your app

### Redeploying

To pull latest changes and redeploy:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-github-app/deploy
```

atmosphere will:
1. `git pull` latest changes
2. Rebuild the application
3. Replace running containers

---

## Manual File Deployment

### Overview

Manual deployment allows you to upload application files directly through the API, useful for:
- Testing deployments
- Apps not in version control
- Quick prototypes

### Step 1: Create App

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-manual-app",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domain": "manual.example.com",
    "port": 80
  }'
```

### Step 2: Upload Files

Upload Dockerfile:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-manual-app/files \
  -F "path=Dockerfile" \
  -F "content=@./Dockerfile"
```

Upload application files:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-manual-app/files \
  -F "path=index.html" \
  -F "content=@./index.html"
```

You can also upload as text:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-manual-app/files \
  -F "path=README.md" \
  -F 'content=# My App

This is my application.'
```

### Step 3: Deploy

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-manual-app/deploy
```

### File Structure

Files are stored in `/opt/atmosphere/workspaces/my-manual-app/`

### Security

- Path traversal is prevented (`.../` not allowed)
- Absolute paths are rejected
- Files must be within app workspace

---

## Docker Compose Deployment

### Overview

Deploy multi-container applications using docker-compose.yml.

### Example docker-compose.yml

```yaml
version: '3.8'

services:
  web:
    build: .
    container_name: ${ATMOSPHERE_APP:-myapp}-web
    restart: unless-stopped
    environment:
      - DATABASE_URL=${DATABASE_URL}
    networks:
      - ${TRAEFIK_NETWORK:-traefik}
      - app-network
    labels:
      - "traefik.enable=true"
      - "traefik.docker.network=${TRAEFIK_NETWORK:-traefik}"
      - "traefik.http.routers.${ATMOSPHERE_APP}.rule=Host(`${DOMAIN}`)"
      - "traefik.http.routers.${ATMOSPHERE_APP}.entrypoints=websecure"
      - "traefik.http.routers.${ATMOSPHERE_APP}.tls=true"
      - "traefik.http.routers.${ATMOSPHERE_APP}.tls.certresolver=letsencrypt"
      - "traefik.http.services.${ATMOSPHERE_APP}.loadbalancer.server.port=3000"
      - "atmosphere.app=${ATMOSPHERE_APP}"

  db:
    image: postgres:15-alpine
    container_name: ${ATMOSPHERE_APP}-db
    restart: unless-stopped
    environment:
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    networks:
      - app-network
    volumes:
      - db-data:/var/lib/postgresql/data

networks:
  app-network:
    driver: bridge
  traefik:
    external: true

volumes:
  db-data:
```

### Important Variables

Atmosphere injects these environment variables:

- `ATMOSPHERE_APP` - Your app name
- `TRAEFIK_NETWORK` - The Traefik network name
- `DOMAIN` - Your configured domain

Use them in your docker-compose.yml for dynamic configuration.

### Deployment

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "fullstack-app",
    "deployment_type": "github",
    "build_type": "compose",
    "github_repo": "git@github.com:user/repo.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/deploy_key)"'",
    "domain": "fullstack.example.com",
    "env_vars": {
      "DB_PASSWORD": "securepassword123"
    }
  }'
```

---

## Dockerfile Deployment

### Overview

Deploy single-container applications using a Dockerfile.

### Example Dockerfile

```dockerfile
FROM node:18-alpine

WORKDIR /app

COPY package*.json ./
RUN npm ci --only=production

COPY . .

EXPOSE 3000

CMD ["node", "server.js"]
```

### Deployment

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "simple-app",
    "deployment_type": "github",
    "build_type": "dockerfile",
    "github_repo": "git@github.com:user/simple-app.git",
    "github_branch": "main",
    "deployment_key": "'"$(cat ~/.ssh/deploy_key)"'",
    "domain": "simple.example.com",
    "port": 3000,
    "env_vars": {
      "NODE_ENV": "production"
    }
  }'
```

### Port Configuration

Atmosphere needs to know which port your container exposes:

```json
{
  "port": 3000
}
```

Default is 8080 if not specified.

---

## Environment Variables

### Setting Environment Variables

When creating an app:
```json
{
  "env_vars": {
    "NODE_ENV": "production",
    "API_KEY": "secret123",
    "DATABASE_URL": "postgresql://..."
  }
}
```

### Updating Environment Variables

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "env_vars": {
      "NODE_ENV": "production",
      "NEW_VAR": "value"
    }
  }'
```

**Note**: Updates replace all env vars. Include existing ones you want to keep.

### Accessing in Containers

Environment variables are passed to containers and accessible normally:

**Node.js**:
```javascript
const apiKey = process.env.API_KEY;
```

**Python**:
```python
import os
api_key = os.environ.get('API_KEY')
```

**Go**:
```go
apiKey := os.Getenv("API_KEY")
```

---

## Domain Configuration

### Setting a Domain

```json
{
  "domain": "myapp.example.com"
}
```

### DNS Configuration

Point your domain to your server IP:

```
A    myapp.example.com    →    YOUR_SERVER_IP
```

Or use a wildcard:
```
A    *.example.com        →    YOUR_SERVER_IP
```

### HTTPS/SSL

Traefik automatically:
1. Obtains SSL certificate from Let's Encrypt
2. Enables HTTPS
3. Redirects HTTP → HTTPS

**Requirements**:
- Domain must be publicly accessible
- Points to your server
- Ports 80 and 443 open
- Valid email in Traefik config

### Updating Domain

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "newdomain.example.com"
  }'
```

Then redeploy:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

---

## Troubleshooting

### Deployment Fails

**Check deployment logs**:
```bash
curl http://localhost:3000/api/v1/apps/my-app/logs
```

**Common issues**:
- Invalid Dockerfile syntax
- Missing dependencies
- Port conflicts
- Network errors

### App Not Accessible

**Check app status**:
```bash
curl http://localhost:3000/api/v1/apps/my-app
```

**Check container**:
```bash
docker ps | grep atmosphere-my-app
docker logs atmosphere-my-app
```

**Check Traefik**:
```bash
cd /opt/traefik
docker compose logs traefik
```

### SSL Certificate Issues

**Check Traefik logs**:
```bash
cd /opt/traefik
docker compose logs | grep acme
```

**Common issues**:
- Port 80 not accessible (required for HTTP challenge)
- Invalid email in Traefik config
- Domain not pointing to server
- Rate limiting from Let's Encrypt

**Check certificates**:
```bash
sudo cat /opt/traefik/acme/acme.json | jq .
```

### GitHub Clone Fails

**Check deployment key**:
```bash
ls -la /opt/atmosphere/keys/
```

Should show `my-app.key` with 0600 permissions.

**Test SSH key**:
```bash
ssh -i /opt/atmosphere/keys/my-app.key -T git@github.com
```

Should output: "Hi username/repo! You've successfully authenticated..."

### Container Won't Start

**Check container logs**:
```bash
docker logs atmosphere-my-app
```

**Check app status**:
```bash
curl http://localhost:3000/api/v1/apps/my-app
```

**Manually start**:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/start
```

### Build Fails

Common causes:
- Invalid Dockerfile syntax
- Missing files in build context
- Network issues downloading dependencies
- Insufficient disk space

**Check disk space**:
```bash
df -h
```

**Clean up Docker**:
```bash
docker system prune -a
```
