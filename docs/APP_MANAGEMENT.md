# App Management Guide

This guide covers day-to-day operations for managing applications in Atmosphere, including deployment monitoring, app lifecycle management, and troubleshooting.

## Table of Contents

1. [Listing Apps](#listing-apps)
2. [Getting App Details](#getting-app-details)
3. [Deploying & Redeploying](#deploying--redeploying)
4. [Monitoring Deployments](#monitoring-deployments)
5. [Starting & Stopping Apps](#starting--stopping-apps)
6. [Updating Apps](#updating-apps)
7. [Deleting Apps](#deleting-apps)
8. [Testing Apps](#testing-apps)
9. [Viewing Logs](#viewing-logs)
10. [Common Workflows](#common-workflows)
11. [Troubleshooting](#troubleshooting)

---

## Listing Apps

### List All Apps

Get a list of all deployed applications:

```bash
curl http://localhost:3000/api/v1/apps
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "my-app",
    "deployment_type": "github",
    "build_type": "compose",
    "status": "running",
    "domain": "app.example.com",
    "github_repo": "git@github.com:user/repo.git",
    "github_branch": "main",
    "port": 3000,
    "created_at": "2026-04-12T10:00:00Z",
    "updated_at": "2026-04-12T15:30:00Z",
    "last_deployed_at": "2026-04-12T15:30:00Z"
  },
  {
    "id": 2,
    "name": "another-app",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "status": "stopped",
    "domain": "another.example.com",
    "created_at": "2026-04-10T08:00:00Z",
    "updated_at": "2026-04-10T08:00:00Z"
  }
]
```

### Format Output with jq

Get just app names and status:

```bash
curl -s http://localhost:3000/api/v1/apps | jq -r '.[] | "\(.name): \(.status)"'
```

**Output:**
```
my-app: running
another-app: stopped
```

Get running apps only:

```bash
curl -s http://localhost:3000/api/v1/apps | jq '.[] | select(.status == "running")'
```

Count total apps:

```bash
curl -s http://localhost:3000/api/v1/apps | jq 'length'
```

### App Status Values

- `running` - App is running and accessible
- `stopped` - App is stopped
- `building` - App is currently being built/deployed
- `failed` - Last deployment failed

---

## Getting App Details

### Get Specific App

```bash
curl http://localhost:3000/api/v1/apps/my-app
```

**Response:**
```json
{
  "id": 1,
  "name": "my-app",
  "deployment_type": "github",
  "build_type": "compose",
  "status": "running",
  "domain": "app.example.com",
  "env_vars": {
    "NODE_ENV": "production",
    "DATABASE_URL": "postgresql://..."
  },
  "github_repo": "git@github.com:user/repo.git",
  "github_branch": "main",
  "compose_path": "docker-compose.prod.yml",
  "port": 3000,
  "created_at": "2026-04-12T10:00:00Z",
  "updated_at": "2026-04-12T15:30:00Z",
  "last_deployed_at": "2026-04-12T15:30:00Z"
}
```

### Extract Specific Fields

Get just the domain:

```bash
curl -s http://localhost:3000/api/v1/apps/my-app | jq -r '.domain'
```

Get deployment information:

```bash
curl -s http://localhost:3000/api/v1/apps/my-app | jq '{
  name: .name,
  type: .deployment_type,
  repo: .github_repo,
  branch: .github_branch,
  status: .status,
  last_deployed: .last_deployed_at
}'
```

---

## Deploying & Redeploying

### Initial Deployment

After creating an app, deploy it:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

**Response:**
```json
{
  "message": "Deployment started",
  "deployment_log": {
    "id": 1,
    "app_id": 1,
    "status": "in_progress",
    "log": "",
    "started_at": "2026-04-12T15:30:00Z"
  }
}
```

### Redeploy (Pull Latest Changes)

For GitHub apps, redeploy to pull latest changes:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

**What happens:**
1. Git pulls latest changes from the specified branch
2. Stops current containers
3. Rebuilds the application
4. Starts new containers
5. Updates Traefik routing

### Zero-Downtime Deployments

Atmosphere doesn't currently support zero-downtime deployments. During redeployment:
- Existing containers are stopped
- New containers are built and started
- Brief downtime occurs (~10-60 seconds depending on build time)

**Best practice:** Deploy during low-traffic periods or use monitoring to track when deployment completes.

---

## Monitoring Deployments

### Check Deployment Status

Get deployment logs to see progress:

```bash
curl http://localhost:3000/api/v1/apps/my-app/logs
```

**Response:**
```json
[
  {
    "id": 5,
    "app_id": 1,
    "status": "success",
    "log": "[15:30:00] Preparing workspace...\n[15:30:01] Cloning repository...\n[15:30:05] Building...\n[15:30:45] Deployment successful!\n",
    "started_at": "2026-04-12T15:30:00Z",
    "ended_at": "2026-04-12T15:30:45Z"
  },
  {
    "id": 4,
    "app_id": 1,
    "status": "failed",
    "log": "[14:00:00] Preparing workspace...\nError: failed to build...",
    "started_at": "2026-04-12T14:00:00Z",
    "ended_at": "2026-04-12T14:00:30Z"
  }
]
```

### View Latest Deployment Only

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq '.[0]'
```

### View Latest Deployment Log Text

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'
```

### Check if Deployment Succeeded

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].status'
```

Returns: `success`, `failed`, or `in_progress`

### Monitor Deployment in Real-Time

Poll deployment status until complete:

```bash
# Simple polling script
while true; do
  STATUS=$(curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].status')
  echo "Status: $STATUS"
  
  if [ "$STATUS" != "in_progress" ]; then
    echo "Deployment completed with status: $STATUS"
    curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'
    break
  fi
  
  sleep 5
done
```

### Limit Deployment History

Get last 5 deployments:

```bash
curl http://localhost:3000/api/v1/apps/my-app/logs?limit=5
```

Get last 20 deployments:

```bash
curl http://localhost:3000/api/v1/apps/my-app/logs?limit=20
```

---

## Starting & Stopping Apps

### Stop an App

Stop all containers for an app:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop
```

**Response:**
```json
{
  "message": "App stopped successfully"
}
```

**What happens:**
- All containers are stopped (but not removed)
- App becomes inaccessible
- Data in volumes is preserved

### Start an App

Start a previously stopped app:

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/start
```

**Response:**
```json
{
  "message": "App started successfully"
}
```

**What happens:**
- Containers are started with their existing configuration
- App becomes accessible again
- No rebuild occurs

### Restart an App

To restart an app (stop then start):

```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop
sleep 2
curl -X POST http://localhost:3000/api/v1/apps/my-app/start
```

### When to Use Stop vs Deploy

| Action | When to Use | Rebuild? | Pull Code? |
|--------|-------------|----------|------------|
| `stop` | Temporarily disable app | No | No |
| `start` | Resume stopped app | No | No |
| `deploy` | Update code or rebuild | Yes | Yes (GitHub) |

---

## Updating Apps

### Update App Configuration

Update app settings (domain, environment variables, etc.):

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "domain": "newdomain.example.com",
    "env_vars": {
      "NODE_ENV": "production",
      "NEW_VAR": "new_value"
    }
  }'
```

**Response:**
```json
{
  "id": 1,
  "name": "my-app",
  "domain": "newdomain.example.com",
  "env_vars": {
    "NODE_ENV": "production",
    "NEW_VAR": "new_value"
  },
  ...
}
```

### What Can Be Updated

You can update these fields:
- `domain` - App domain name
- `env_vars` - Environment variables (**replaces all**)
- `github_branch` - GitHub branch to deploy from
- `github_subdir` - Subdirectory in repo
- `compose_path` - Docker Compose file path
- `dockerfile_path` - Dockerfile path
- `port` - Container port

### Apply Configuration Changes

After updating configuration, redeploy to apply changes:

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "env_vars": {
      "NODE_ENV": "production",
      "API_KEY": "new-key"
    }
  }'

# Redeploy to apply changes
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Update GitHub Branch

Switch to a different branch:

```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"github_branch": "staging"}'

# Deploy from new branch
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Important Notes

⚠️ **Environment Variables:** Updating `env_vars` **replaces all existing variables**. Always include all variables you want to keep:

```bash
# ❌ Wrong - This removes all other env vars
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"env_vars": {"NEW_VAR": "value"}}'

# ✅ Correct - Include all vars you want
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "env_vars": {
      "NODE_ENV": "production",
      "DATABASE_URL": "postgresql://...",
      "NEW_VAR": "value"
    }
  }'
```

---

## Deleting Apps

### Delete an App

Permanently delete an app and all its resources:

```bash
curl -X DELETE http://localhost:3000/api/v1/apps/my-app
```

**Response:**
```json
{
  "message": "App deleted successfully"
}
```

### What Gets Deleted

When you delete an app:
- ✅ App record removed from database
- ✅ Containers stopped and removed
- ✅ Images removed
- ✅ Networks removed
- ✅ SSH deployment key deleted
- ⚠️ **Volumes preserved** (data safety)
- ⚠️ **Workspace directory preserved**

### Clean Up Volumes Manually

If you want to delete volumes too:

```bash
# List volumes for the app
docker volume ls | grep my-app

# Delete specific volume
docker volume rm my-app-db-data

# Or delete all unused volumes
docker volume prune
```

### Clean Up Workspace

Remove workspace directory manually:

```bash
rm -rf /opt/atmosphere/workspaces/my-app
```

### Before Deleting

**Consider:**
- Export any data you need from volumes
- Back up important files from workspace
- Note environment variables if you'll recreate the app

---

## Testing Apps

### Health Check

Test if app is responding:

```bash
curl -I https://app.example.com
```

**Expected:**
```
HTTP/2 200
```

### Test with Specific Domain

If DNS isn't set up yet, test with Host header:

```bash
curl -H "Host: app.example.com" http://SERVER_IP
```

### Test SSL Certificate

Check if HTTPS is working:

```bash
curl -v https://app.example.com 2>&1 | grep -i "SSL certificate"
```

### Test from Different Location

```bash
curl -I https://app.example.com
```

From another server or your local machine to verify external access.

### Check App Response Time

```bash
curl -o /dev/null -s -w "Time: %{time_total}s\n" https://app.example.com
```

### Verify Container is Running

```bash
docker ps | grep atmosphere-my-app
```

**Expected output:**
```
abc123...  atmosphere-my-app  Up 5 minutes
```

### Check Container Logs

```bash
docker logs atmosphere-my-app
```

Or follow logs in real-time:

```bash
docker logs -f atmosphere-my-app
```

### Test Database Connection (for compose apps)

```bash
# Example for PostgreSQL
docker exec atmosphere-my-app-db psql -U postgres -c "SELECT version();"
```

### Load Testing

Simple load test with Apache Bench:

```bash
ab -n 1000 -c 10 https://app.example.com/
```

- `-n 1000` - 1000 requests total
- `-c 10` - 10 concurrent requests

---

## Viewing Logs

### Deployment Logs

See deployment history and build logs:

```bash
curl http://localhost:3000/api/v1/apps/my-app/logs
```

### Latest Deployment Log

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'
```

### Only Failed Deployments

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq '.[] | select(.status == "failed")'
```

### Only Successful Deployments

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq '.[] | select(.status == "success")'
```

### Deployment Logs from Last 24 Hours

```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq --arg since "$(date -u -d '24 hours ago' +%Y-%m-%dT%H:%M:%SZ)" '.[] | select(.started_at > $since)'
```

### Container Runtime Logs

View live application logs:

```bash
docker logs atmosphere-my-app
```

Follow logs in real-time:

```bash
docker logs -f atmosphere-my-app
```

Last 100 lines:

```bash
docker logs --tail 100 atmosphere-my-app
```

Logs since 1 hour ago:

```bash
docker logs --since 1h atmosphere-my-app
```

### Multi-Container Apps (Compose)

View logs from all containers:

```bash
cd /opt/atmosphere/workspaces/my-app
docker compose logs
```

Follow all logs:

```bash
docker compose logs -f
```

Logs from specific service:

```bash
docker compose logs web
docker compose logs db
```

### Search Logs for Errors

```bash
docker logs atmosphere-my-app 2>&1 | grep -i error
docker logs atmosphere-my-app 2>&1 | grep -i warning
```

---

## Common Workflows

### Deploy New Feature from GitHub

```bash
# 1. Push code to GitHub
git push origin main

# 2. Trigger deployment
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy

# 3. Monitor progress
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].status'

# 4. Check deployment log if failed
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'

# 5. Verify app is running
curl -I https://app.example.com
```

### Update Environment Variables

```bash
# 1. Get current config (to preserve existing vars)
curl -s http://localhost:3000/api/v1/apps/my-app | jq '.env_vars' > current_vars.json

# 2. Edit vars file manually or use jq
cat current_vars.json | jq '. + {"NEW_VAR": "value"}' > updated_vars.json

# 3. Update app
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d "{\"env_vars\": $(cat updated_vars.json)}"

# 4. Redeploy to apply changes
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Rollback to Previous Version

```bash
# 1. Stop current deployment
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop

# 2. In workspace, git reset to previous commit
cd /opt/atmosphere/workspaces/my-app
git log --oneline -5  # Find previous commit
git reset --hard <commit-hash>

# 3. Redeploy
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Switch Between Staging and Production

```bash
# Deploy staging branch
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"github_branch": "staging"}'

curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy

# Later, switch to production
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"github_branch": "main"}'

curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Debug Failed Deployment

```bash
# 1. Check deployment logs
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'

# 2. Check app details
curl -s http://localhost:3000/api/v1/apps/my-app | jq

# 3. Manually inspect workspace
ls -la /opt/atmosphere/workspaces/my-app/

# 4. Try to build manually
cd /opt/atmosphere/workspaces/my-app
docker compose build  # or docker build

# 5. Check for resource issues
docker system df
df -h

# 6. After fixing, redeploy
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Migrate App to New Domain

```bash
# 1. Update domain
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"domain": "newdomain.example.com"}'

# 2. Update DNS to point to server
# (Do this manually in your DNS provider)

# 3. Redeploy to update Traefik labels
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy

# 4. Wait for Let's Encrypt certificate (1-2 minutes)

# 5. Test new domain
curl -I https://newdomain.example.com
```

---

## Troubleshooting

### App Shows as Running but Not Accessible

**Check container status:**
```bash
docker ps | grep atmosphere-my-app
```

**Check container logs:**
```bash
docker logs atmosphere-my-app
```

**Check Traefik routing:**
```bash
cd /opt/traefik
docker compose logs | grep my-app
```

💡 **See also:** [Traefik Guide](TRAEFIK_GUIDE.md) for detailed Traefik management and troubleshooting.

**Verify domain DNS:**
```bash
nslookup app.example.com
dig app.example.com
```

**Test direct access to container:**
```bash
# Get container IP
docker inspect atmosphere-my-app | jq -r '.[0].NetworkSettings.Networks.traefik.IPAddress'

# Try accessing directly
curl http://<container-ip>:3000
```

### Deployment Stuck in "in_progress"

**Check Atmosphere backend logs:**
```bash
journalctl -u atmosphere -n 100 --no-pager
```

**Check if deployment process is still running:**
```bash
ps aux | grep deploy
```

**Try restarting Atmosphere:**
```bash
systemctl restart atmosphere
```

**Check deployment logs for last output:**
```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'
```

### Can't Delete App

**Stop containers first:**
```bash
docker stop $(docker ps -q --filter "label=atmosphere.app=my-app")
docker rm $(docker ps -aq --filter "label=atmosphere.app=my-app")
```

**Then try deleting:**
```bash
curl -X DELETE http://localhost:3000/api/v1/apps/my-app
```

### App Won't Start After Update

**Check latest deployment log:**
```bash
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'
```

**Try manual start:**
```bash
cd /opt/atmosphere/workspaces/my-app
docker compose up -d
```

**Check for port conflicts:**
```bash
docker ps  # See if another container uses the same port
```

### Environment Variables Not Applied

**Verify variables are saved:**
```bash
curl -s http://localhost:3000/api/v1/apps/my-app | jq '.env_vars'
```

**Remember to redeploy after updating:**
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

**Check if containers received the variables:**
```bash
docker inspect atmosphere-my-app | jq '.[0].Config.Env'
```

### SSL Certificate Not Generated

💡 **For detailed SSL/certificate troubleshooting, see:** [Traefik Guide - Let's Encrypt & SSL Certificates](TRAEFIK_GUIDE.md#lets-encrypt--ssl-certificates)

**Check Traefik logs:**
```bash
cd /opt/traefik
docker compose logs | grep -i acme
docker compose logs | grep -i certificate
```

**Verify domain points to server:**
```bash
dig app.example.com +short
```

**Check Traefik email is valid:**
```bash
cat /opt/traefik/traefik.yml | grep email
```

**Manually trigger certificate:**
```bash
# Restart Traefik
cd /opt/traefik
docker compose restart

# Wait 1-2 minutes and test
curl -I https://app.example.com
```

### Out of Disk Space

**Check disk usage:**
```bash
df -h
```

**Clean up Docker:**
```bash
# Remove unused images
docker image prune -a

# Remove unused volumes
docker volume prune

# Remove everything unused
docker system prune -a --volumes
```

**Check largest directories:**
```bash
du -sh /opt/atmosphere/workspaces/*
du -sh /var/lib/docker/*
```

---

## Quick Reference

### Essential Commands

```bash
# List all apps
curl http://localhost:3000/api/v1/apps

# Get app details
curl http://localhost:3000/api/v1/apps/my-app

# Deploy/redeploy
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy

# Check deployment status
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].status'

# View deployment log
curl -s http://localhost:3000/api/v1/apps/my-app/logs | jq -r '.[0].log'

# Stop app
curl -X POST http://localhost:3000/api/v1/apps/my-app/stop

# Start app
curl -X POST http://localhost:3000/api/v1/apps/my-app/start

# Update app
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{"domain": "newdomain.com"}'

# Delete app
curl -X DELETE http://localhost:3000/api/v1/apps/my-app

# Container logs
docker logs -f atmosphere-my-app

# Test app
curl -I https://app.example.com
```

### Status Codes

| HTTP Status | Meaning |
|-------------|---------|
| 200 OK | Success |
| 201 Created | App created |
| 202 Accepted | Deployment started |
| 400 Bad Request | Invalid input |
| 404 Not Found | App doesn't exist |
| 500 Internal Server Error | Server error |

---

## Best Practices

1. **Monitor deployments** - Always check deployment logs after deploying
2. **Test before deploying** - Test changes locally with Docker first
3. **Use environment variables** - Never hardcode secrets in code
4. **Keep backups** - Export important data before updates
5. **Deploy off-peak** - Minimize impact of deployment downtime
6. **Check logs regularly** - Catch issues early
7. **Clean up periodically** - Remove old images and volumes
8. **Document custom configs** - Note non-standard settings
9. **Use version control** - Always deploy from Git when possible
10. **Test after deployment** - Verify app works after each deploy
