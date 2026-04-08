# Docker Compose Conversion Guide
## Converting Local Docker Setup for Atmosphere Deployment

This guide helps you restructure your existing Docker configuration to support both **local development** (Docker Desktop) and **Atmosphere deployment** using Docker Compose's override pattern.

---

## 📋 Table of Contents

1. [Overview](#overview)
2. [File Structure](#file-structure)
3. [Step-by-Step Conversion](#step-by-step-conversion)
4. [Testing](#testing)
5. [Atmosphere Deployment](#atmosphere-deployment)
6. [Common Patterns](#common-patterns)
7. [Troubleshooting](#troubleshooting)

---

## Overview

### The Problem
You have a `docker-compose.yml` for local development with:
- Exposed ports for direct access
- Volume mounts for live code reloading
- Development environment variables
- No reverse proxy configuration

You need to deploy to Atmosphere which requires:
- Traefik labels for routing (no exposed ports)
- External Traefik network
- Production-optimized settings
- Atmosphere environment variables

### The Solution
Split your configuration into **three files**:
1. **`docker-compose.yml`** - Base configuration (shared by both environments)
2. **`docker-compose.override.yml`** - Local development overrides (auto-loaded)
3. **`docker-compose.prod.yml`** - Atmosphere/production overrides (explicit)

---

## File Structure

### Before (Single File)
```
your-app/
├── docker-compose.yml          # Everything in one file
├── Dockerfile
└── .env
```

### After (Three Files)
```
your-app/
├── docker-compose.yml          # Base config (shared)
├── docker-compose.override.yml # Local dev (auto-applied)
├── docker-compose.prod.yml     # Atmosphere (explicit)
├── Dockerfile
├── .env                        # Local environment
└── .env.example                # Template for production
```

---

## Step-by-Step Conversion

### Step 1: Backup Your Current Setup

```bash
# Make a backup
cp docker-compose.yml docker-compose.yml.backup
```

### Step 2: Identify Configuration Categories

Review your current `docker-compose.yml` and categorize each section:

| Configuration | Goes In | Example |
|--------------|---------|---------|
| Service name, image, build | **Base** | `services:`, `build:`, `image:` |
| Container name | **Base** | `container_name: ${ATMOSPHERE_APP:-myapp}` |
| Restart policy | **Base** | `restart: unless-stopped` |
| **Port mappings** | **Override (local)** | `ports: - "8080:80"` |
| **Volume mounts (code)** | **Override (local)** | `volumes: - ./:/app` |
| Volume mounts (data) | **Base** | `volumes: - ./data:/data` |
| Basic environment | **Base** | Common env vars |
| Dev environment | **Override (local)** | `NODE_ENV=development` |
| **Traefik labels** | **Prod** | All `traefik.*` labels |
| **Traefik network** | **Prod** | `networks: - traefik` |
| Internal networks | **Base** | `networks: - app-network` |
| Resource limits | **Prod** | `deploy.resources` |

### Step 3: Create Base Configuration

Create **`docker-compose.yml`** with **only shared configuration**:

```yaml
# docker-compose.yml - Base configuration for all environments
services:
  web:
    build:
      context: .
      dockerfile: Dockerfile
      # Build args that are common to both environments
      args:
        - APP_VERSION=${APP_VERSION:-latest}
    
    container_name: ${ATMOSPHERE_APP:-myapp}-web
    restart: unless-stopped
    
    # Working directory
    working_dir: /var/www/html
    
    # Volumes that persist in both environments (logs, data, etc.)
    volumes:
      - ./logs:/var/www/html/logs
      - ./data:/var/www/html/data
    
    # Internal network (always needed)
    networks:
      - app-network
    
    # Load environment file
    env_file:
      - .env
    
    # Health check (good for both envs)
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  # Additional services (database, redis, etc.)
  redis:
    image: redis:alpine
    container_name: ${ATMOSPHERE_APP:-myapp}-redis
    restart: unless-stopped
    networks:
      - app-network
    volumes:
      - redis-data:/data

# Define internal network
networks:
  app-network:
    driver: bridge

# Named volumes
volumes:
  redis-data:
```

### Step 4: Create Local Development Override

Create **`docker-compose.override.yml`** with **local-specific configuration**:

```yaml
# docker-compose.override.yml - Local development (auto-loaded by Docker Compose)
services:
  web:
    # Expose ports for local access (not needed in production)
    ports:
      - "${WEB_PORT:-8080}:80"
    
    # Mount source code for live reloading
    volumes:
      - ./:/var/www/html              # Mount entire project
      - /var/www/html/vendor          # Protect vendor directory
      - /var/www/html/node_modules    # Protect node_modules
    
    # Development environment variables
    environment:
      - NODE_ENV=development
      - DEBUG=true
      - LOG_LEVEL=debug
      - APP_ENV=local
    
    # No resource limits in local development
```

**Important Notes:**
- This file is **automatically loaded** by Docker Compose when you run `docker compose up`
- Perfect for development settings that should never go to production
- Can be added to `.gitignore` if it contains machine-specific paths

### Step 5: Create Atmosphere/Production Configuration

Create **`docker-compose.prod.yml`** with **Atmosphere-specific configuration**:

```yaml
# docker-compose.prod.yml - Atmosphere deployment
services:
  web:
    # NO PORT MAPPINGS - Traefik handles all routing
    
    # Connect to Traefik network (must be external)
    networks:
      - app-network
      - ${TRAEFIK_NETWORK:-traefik}
    
    # Traefik labels for automatic routing and SSL
    labels:
      - "traefik.enable=true"
      - "traefik.docker.network=${TRAEFIK_NETWORK:-traefik}"
      
      # Router configuration
      - "traefik.http.routers.${ATMOSPHERE_APP}.rule=Host(`${DOMAIN}`)"
      - "traefik.http.routers.${ATMOSPHERE_APP}.entrypoints=websecure"
      - "traefik.http.routers.${ATMOSPHERE_APP}.tls=true"
      - "traefik.http.routers.${ATMOSPHERE_APP}.tls.certresolver=letsencrypt"
      
      # Service configuration
      - "traefik.http.services.${ATMOSPHERE_APP}.loadbalancer.server.port=80"
      
      # Atmosphere tracking label
      - "atmosphere.app=${ATMOSPHERE_APP}"
    
    # Production environment overrides
    environment:
      - NODE_ENV=production
      - DEBUG=false
      - LOG_LEVEL=info
    
    # Resource limits for production
    deploy:
      resources:
        limits:
          cpus: '${CONTAINER_CPU_LIMIT:-2.0}'
          memory: ${CONTAINER_MEMORY_LIMIT:-1G}
        reservations:
          cpus: '${CONTAINER_CPU_RESERVATION:-0.5}'
          memory: ${CONTAINER_MEMORY_RESERVATION:-256M}

  # Redis doesn't need Traefik exposure (internal only)
  redis:
    # No changes needed - internal services stay the same

# Traefik network must be external (created by Atmosphere)
networks:
  traefik:
    external: true
```

### Step 6: Update Environment Files

Create **`.env.example`** for production template:

```bash
# .env.example - Template for production deployment

# Application
APP_NAME=myapp
APP_ENV=production
APP_DEBUG=false

# These are injected by Atmosphere automatically:
# ATMOSPHERE_APP=myapp
# DOMAIN=myapp.yourdomain.com
# TRAEFIK_NETWORK=traefik

# Database
DB_HOST=db
DB_PORT=5432
DB_NAME=myapp
DB_USER=myapp
DB_PASSWORD=change_me_in_production

# Redis
REDIS_HOST=redis
REDIS_PORT=6379

# Resource limits (optional)
CONTAINER_CPU_LIMIT=2.0
CONTAINER_MEMORY_LIMIT=1G
CONTAINER_CPU_RESERVATION=0.5
CONTAINER_MEMORY_RESERVATION=256M
```

Keep your local **`.env`** for development:

```bash
# .env - Local development

APP_NAME=myapp
APP_ENV=local
APP_DEBUG=true
WEB_PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_NAME=myapp_dev
DB_USER=dev
DB_PASSWORD=dev

REDIS_HOST=localhost
REDIS_PORT=6379
```

### Step 7: Update .gitignore

```gitignore
# Environment files
.env

# Keep the example in git
!.env.example

# Optionally ignore local override if machine-specific
# docker-compose.override.yml

# Logs and data
logs/
data/

# Docker volumes
volumes/
```

---

## Testing

### Test Local Development

```bash
# This automatically uses docker-compose.yml + docker-compose.override.yml
docker compose up -d

# Verify services are running
docker compose ps

# Check logs
docker compose logs -f web

# Access your app
curl http://localhost:8080
# Or open in browser: http://localhost:8080
```

**Verify:**
- ✅ Ports are exposed (can access via localhost)
- ✅ Code changes reflect immediately (no rebuild needed)
- ✅ Development environment variables are set

### Test Production Configuration Locally

```bash
# Explicitly use production config
docker compose -f docker-compose.yml -f docker-compose.prod.yml config

# This shows the merged configuration - review it!
```

**Check for:**
- ✅ No exposed ports
- ✅ Traefik labels present
- ✅ Traefik network connected
- ✅ Resource limits applied

### Clean Up

```bash
# Stop and remove everything
docker compose down -v

# Remove unused volumes
docker volume prune
```

---

## Atmosphere Deployment

### Method 1: GitHub Deployment (Recommended)

1. **Push your code to GitHub** (including all three compose files)

2. **Generate SSH deployment key**:
   ```bash
   ssh-keygen -t ed25519 -C "atmosphere-myapp" -f ~/.ssh/atmosphere_myapp
   ```

3. **Add public key to GitHub**:
   - Repository → Settings → Deploy keys
   - Add `~/.ssh/atmosphere_myapp.pub`

4. **Deploy to Atmosphere**:
   ```bash
   curl -X POST http://your-atmosphere-server:3000/api/v1/apps \
     -H "Content-Type: application/json" \
     -d '{
       "name": "myapp",
       "deployment_type": "github",
       "build_type": "compose",
       "compose_file": "docker-compose.prod.yml",
       "github_repo": "git@github.com:yourusername/your-repo.git",
       "github_branch": "main",
       "deployment_key": "'"$(cat ~/.ssh/atmosphere_myapp)"'",
       "domain": "myapp.yourdomain.com",
       "env_vars": {
         "APP_ENV": "production",
         "DB_PASSWORD": "secure_password_here",
         "SECRET_KEY": "your_secret_key"
       }
     }'
   ```

**Key points:**
- ✅ Specify `"compose_file": "docker-compose.prod.yml"`
- ✅ Atmosphere will use `docker-compose.yml` + `docker-compose.prod.yml`
- ✅ Local `docker-compose.override.yml` is **ignored** (not specified)

### Method 2: Manual File Upload

1. **Create deployment package**:
   ```bash
   # Create a clean copy without dev files
   mkdir deploy
   cp docker-compose.yml deploy/
   cp docker-compose.prod.yml deploy/
   cp Dockerfile deploy/
   cp .env.example deploy/.env
   cp -r src deploy/
   
   # Create tarball
   cd deploy
   tar -czf ../myapp-deploy.tar.gz .
   cd ..
   ```

2. **Upload to Atmosphere** via web UI or API

---

## Common Patterns

### Pattern 1: Multiple Services with Traefik

```yaml
# docker-compose.prod.yml
services:
  web:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.${ATMOSPHERE_APP}-web.rule=Host(`${DOMAIN}`)"
      - "traefik.http.services.${ATMOSPHERE_APP}-web.loadbalancer.server.port=80"
    networks:
      - traefik
      - app-network

  api:
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.${ATMOSPHERE_APP}-api.rule=Host(`${DOMAIN}`) && PathPrefix(`/api`)"
      - "traefik.http.services.${ATMOSPHERE_APP}-api.loadbalancer.server.port=3000"
    networks:
      - traefik
      - app-network
  
  # Database - internal only (no Traefik)
  db:
    networks:
      - app-network
```

### Pattern 2: PHP with Volume Mounting Strategy

```yaml
# docker-compose.yml (base)
services:
  web:
    volumes:
      # Logs always mounted
      - ./logs:/var/www/html/logs

# docker-compose.override.yml (local)
services:
  web:
    volumes:
      # Mount source code for development
      - ./:/var/www/html
      - /var/www/html/vendor

# docker-compose.prod.yml (production)
# In production, code is baked into the image during build
# No source code volume mounts needed
```

### Pattern 3: Build Arguments by Environment

```yaml
# docker-compose.yml (base)
services:
  web:
    build:
      context: .
      args:
        - BASE_IMAGE=${BASE_IMAGE:-php:8.3-apache}

# docker-compose.override.yml (local)
services:
  web:
    build:
      args:
        - ENABLE_XDEBUG=true
        - INSTALL_DEV_TOOLS=true

# docker-compose.prod.yml (production)
services:
  web:
    build:
      args:
        - ENABLE_XDEBUG=false
        - INSTALL_DEV_TOOLS=false
```

### Pattern 4: Multiple Domains/Subdomains

```yaml
# docker-compose.prod.yml
services:
  web:
    labels:
      # Support multiple domains
      - "traefik.http.routers.${ATMOSPHERE_APP}.rule=Host(`${DOMAIN}`) || Host(`www.${DOMAIN}`)"
      
      # Or subdomain
      - "traefik.http.routers.${ATMOSPHERE_APP}-admin.rule=Host(`admin.${DOMAIN_ROOT}`)"
```

---

## Troubleshooting

### Issue: "Network traefik declared as external, but could not be found"

**Local Development:**
```bash
# Create the network locally for testing
docker network create traefik
```

**Or** remove Traefik network from local testing:
```bash
# Test without external network
docker compose -f docker-compose.yml -f docker-compose.override.yml up -d
```

### Issue: "Port already allocated"

Check if something is using the port:
```bash
# Windows
netstat -ano | findstr :8080

# Find and kill the process
taskkill /PID <PID> /F
```

Or change the port in `.env`:
```bash
WEB_PORT=8081
```

### Issue: Code changes not reflecting in local development

Verify override file is being loaded:
```bash
docker compose config | grep -A 5 "volumes:"
```

Should show your source code mount:
```yaml
volumes:
  - ./:/var/www/html
```

If not, ensure `docker-compose.override.yml` exists and run:
```bash
docker compose down
docker compose up -d
```

### Issue: Atmosphere deployment fails to start

1. **Check if base + prod configs merge correctly:**
   ```bash
   docker compose -f docker-compose.yml -f docker-compose.prod.yml config > merged.yml
   cat merged.yml
   ```

2. **Verify required environment variables:**
   - `ATMOSPHERE_APP` - injected by Atmosphere
   - `DOMAIN` - injected by Atmosphere
   - `TRAEFIK_NETWORK` - injected by Atmosphere

3. **Check Atmosphere logs:**
   ```bash
   # On Atmosphere server
   sudo journalctl -u atmosphere -f
   ```

### Issue: Can't access app after Atmosphere deployment

1. **Verify Traefik is running:**
   ```bash
   docker ps | grep traefik
   ```

2. **Check container logs:**
   ```bash
   docker logs <container-name>
   ```

3. **Verify Traefik labels:**
   ```bash
   docker inspect <container-name> | grep -A 20 "Labels"
   ```

4. **Check DNS settings:**
   ```bash
   nslookup myapp.yourdomain.com
   ```

---

## Checklist

### Before Conversion
- [ ] Backup current `docker-compose.yml`
- [ ] Test current setup works
- [ ] Document current port mappings
- [ ] Note all environment variables

### During Conversion
- [ ] Create `docker-compose.yml` (base)
- [ ] Create `docker-compose.override.yml` (local)
- [ ] Create `docker-compose.prod.yml` (atmosphere)
- [ ] Update `.env` for local
- [ ] Create `.env.example` for production
- [ ] Update `.gitignore`

### Testing
- [ ] Local dev works: `docker compose up -d`
- [ ] Can access via localhost:PORT
- [ ] Code changes reflect without rebuild
- [ ] Production config validates: `docker compose -f docker-compose.yml -f docker-compose.prod.yml config`
- [ ] No ports exposed in prod config
- [ ] Traefik labels present in prod config

### Deployment
- [ ] Code pushed to GitHub (if using GitHub deploy)
- [ ] SSH key added to repository
- [ ] Atmosphere app created with `compose_file: "docker-compose.prod.yml"`
- [ ] App deployed successfully
- [ ] Can access via domain
- [ ] SSL certificate obtained

---

## Additional Resources

- [Docker Compose Override Documentation](https://docs.docker.com/compose/multiple-compose-files/extends/)
- [Traefik Docker Provider](https://doc.traefik.io/traefik/providers/docker/)
- [Atmosphere Deployment Guide](./DEPLOYMENT_GUIDE.md)
- [Atmosphere Workflow](./WORKFLOW.md)

---

## Quick Reference

### Local Development Commands
```bash
# Start (uses base + override automatically)
docker compose up -d

# Stop
docker compose down

# Rebuild
docker compose up -d --build

# View logs
docker compose logs -f

# View merged config
docker compose config
```

### Production Config Testing Commands
```bash
# View merged prod config (don't start)
docker compose -f docker-compose.yml -f docker-compose.prod.yml config

# Test prod config locally (if Traefik network exists)
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

# Stop prod config
docker compose -f docker-compose.yml -f docker-compose.prod.yml down
```

### Atmosphere Deployment Commands
```bash
# Deploy with GitHub
curl -X POST http://atmosphere:3000/api/v1/apps -H "Content-Type: application/json" -d '{...}'

# Trigger redeployment
curl -X POST http://atmosphere:3000/api/v1/apps/{app-id}/deploy

# Check app status
curl http://atmosphere:3000/api/v1/apps/{app-id}

# View deployment logs
curl http://atmosphere:3000/api/v1/apps/{app-id}/logs
```

---

**Last Updated:** April 8, 2026  
**Atmosphere Version:** 1.0
