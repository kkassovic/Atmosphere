# Updating Atmosphere

This guide explains how to update your Atmosphere installation to the latest version.

## Table of Contents

1. [Quick Update](#quick-update)
2. [Step-by-Step Update](#step-by-step-update)
3. [Verifying the Update](#verifying-the-update)
4. [Rollback](#rollback-to-previous-version)
5. [Troubleshooting](#troubleshooting)

---

## Quick Update

For experienced users who just need the commands:

```bash
# 1. Navigate to source directory
cd ~/atmosphere

# 2. Pull latest changes
git pull origin main

# 3. Rebuild binary to correct location
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# 4. Restart service
systemctl restart atmosphere

# 5. Verify
systemctl status atmosphere
journalctl -u atmosphere -n 50
```

---

## Step-by-Step Update

### Prerequisites

- SSH access to your Atmosphere server
- Root or sudo privileges
- Atmosphere installed via official installer

### Step 1: Backup Current Version

Always backup before updating:

```bash
# Backup current binary
sudo cp /opt/atmosphere/atmosphere /opt/atmosphere/atmosphere.backup.$(date +%Y%m%d)

# Backup database
sudo cp /opt/atmosphere/atmosphere.db /opt/atmosphere/atmosphere.db.backup.$(date +%Y%m%d)

# Backup environment file
sudo cp /opt/atmosphere/.env /opt/atmosphere/.env.backup.$(date +%Y%m%d)
```

**Verify backups:**
```bash
ls -lh /opt/atmosphere/*.backup.*
```

### Step 2: Check Current Version

Before updating, note your current version:

```bash
# Check running binary location
lsof -p $(pgrep atmosphere) | grep atmosphere

# View service status
systemctl status atmosphere

# Check recent logs
journalctl -u atmosphere -n 20
```

### Step 3: Pull Latest Code

Navigate to the source directory and pull updates:

```bash
cd ~/atmosphere

# Check current branch
git branch

# Fetch latest changes
git fetch origin

# View what's changed (optional)
git log HEAD..origin/main --oneline

# Pull updates
git pull origin main
```

**Expected output:**
```
Updating <hash>..<hash>
Fast-forward
 backend/internal/services/deployment_service.go | 42 ++++++++++++++++++++
 docs/UPDATING.md                                 | 150 ++++++++++++++++++++
 2 files changed, 192 insertions(+)
```

### Step 4: Check for Dependency Changes

If `go.mod` was updated, fetch new dependencies:

```bash
# Navigate to backend directory
cd ~/atmosphere/backend

# Check if go.mod changed
git diff HEAD@{1} go.mod

# If changed, update dependencies
go mod download
go mod tidy
```

### Step 5: Rebuild the Binary

**Critical:** Build to the correct location!

The service runs from `/opt/atmosphere/atmosphere` (NOT `/usr/local/bin/atmosphere`).

```bash
# Build to correct location (from backend directory)
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Verify binary was created
ls -lh /opt/atmosphere/atmosphere

# Check binary info
file /opt/atmosphere/atmosphere
```

**Expected output:**
```
-rwxr-xr-x 1 root root 15M Apr 12 15:30 /opt/atmosphere/atmosphere
/opt/atmosphere/atmosphere: ELF 64-bit LSB executable, x86-64
```

### Step 6: Verify Configuration

Check if any new configuration options were added:

```bash
# Compare .env with example
diff /opt/atmosphere/.env ~/atmosphere/.env.example
```

If new variables were added, update your `.env`:

```bash
sudo nano /opt/atmosphere/.env
# Add any new required variables
```

### Step 7: Restart the Service

```bash
# Restart Atmosphere service
sudo systemctl restart atmosphere

# Wait a moment for startup
sleep 2

# Check status
sudo systemctl status atmosphere
```

**Expected output:**
```
● atmosphere.service - Atmosphere Deployment Platform
     Loaded: loaded (/etc/systemd/system/atmosphere.service; enabled)
     Active: active (running) since Sat 2026-04-12 15:35:23 UTC; 3s ago
```

### Step 8: Verify the Update

Check that Atmosphere is running correctly:

```bash
# Check service logs
journalctl -u atmosphere -n 50 -f

# Test API health
curl http://localhost:3000/health

# List apps (should still work)
curl http://localhost:3000/api/v1/apps
```

**Expected health response:**
```json
{"status":"ok"}
```

---

## Verifying the Update

### Check Running Process

Verify the correct binary is running:

```bash
# Should show /opt/atmosphere/atmosphere
lsof -p $(pgrep atmosphere) | grep atmosphere
```

**Expected output:**
```
atmospher 152663 root  txt    REG  252,1 15728640 123456 /opt/atmosphere/atmosphere
```

### Test Deployments

Test that deployments still work:

```bash
# Get an existing app
curl http://localhost:3000/api/v1/apps/your-app-name

# Trigger a deployment
curl -X POST http://localhost:3000/api/v1/apps/your-app-name/deploy

# Check deployment logs
curl http://localhost:3000/api/v1/apps/your-app-name/logs
```

### Check Traefik Integration

Verify apps are still accessible:

```bash
# Test HTTPS access to an existing app
curl -I https://your-app.yourdomain.com

# Check Traefik can see containers
docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Labels}}" | grep atmosphere
```

---

## Rollback to Previous Version

If the update causes issues, you can rollback:

### Quick Rollback

```bash
# Stop current service
sudo systemctl stop atmosphere

# Restore backup binary
sudo cp /opt/atmosphere/atmosphere.backup.YYYYMMDD /opt/atmosphere/atmosphere

# Restore backup database (if needed)
sudo cp /opt/atmosphere/atmosphere.db.backup.YYYYMMDD /opt/atmosphere/atmosphere.db

# Restart service
sudo systemctl start atmosphere

# Verify
systemctl status atmosphere
```

### Full Rollback (with source code)

```bash
# Navigate to source
cd ~/atmosphere

# Find the previous commit
git log --oneline -10

# Reset to previous version
git reset --hard <previous-commit-hash>

# Rebuild
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Restart
sudo systemctl restart atmosphere
```

---

## Troubleshooting

### Issue: Service fails to start after update

**Check logs:**
```bash
journalctl -u atmosphere -n 100 --no-pager
```

**Common causes:**
- Binary built to wrong location
- Missing configuration variables
- Database schema changes (rare)

**Solution:**
```bash
# Verify binary location
ls -lh /opt/atmosphere/atmosphere

# Check which binary the service tries to run
grep ExecStart /etc/systemd/system/atmosphere.service

# Rebuild to correct location if needed
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Restart
sudo systemctl restart atmosphere
```

### Issue: "text file busy" when rebuilding

**Cause:** Cannot overwrite running binary.

**Solution:**
```bash
# Stop service first
sudo systemctl stop atmosphere

# Rebuild
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Start service
sudo systemctl start atmosphere
```

### Issue: Build fails with "go: command not found"

**Cause:** Go is not in PATH.

**Solution:**
```bash
# Find Go installation
which go
# If not found, check common locations
ls /usr/local/go/bin/go

# Add to PATH temporarily
export PATH=$PATH:/usr/local/go/bin

# Then rebuild
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Make permanent (add to .bashrc)
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
```

### Issue: Apps stop working after update

**Check:**
1. **Traefik still running:**
   ```bash
   docker ps | grep traefik
   ```

2. **Networks still exist:**
   ```bash
   docker network ls | grep -E 'atmosphere|traefik'
   ```

3. **Container labels correct:**
   ```bash
   docker inspect <container-name> | grep -A 20 Labels
   ```

**Solution:**
Redeploy affected apps:
```bash
curl -X POST http://localhost:3000/api/v1/apps/<app-name>/deploy
```

### Issue: Database migration errors

**Symptom:**
```
Error: failed to migrate database
```

**Solution:**
Check if schema changed:
```bash
cd ~/atmosphere
git log --oneline -20 | grep -i migration
```

Usually migrations are automatic. If manual intervention needed, check release notes.

---

## Update Checklist

Before updating:
- [ ] Backup binary: `/opt/atmosphere/atmosphere`
- [ ] Backup database: `/opt/atmosphere/atmosphere.db`
- [ ] Backup environment: `/opt/atmosphere/.env`
- [ ] Note current version/commit: `git log -1`
- [ ] Check active apps: `curl http://localhost:3000/api/v1/apps`

During update:
- [ ] Pull latest code: `git pull origin main`
- [ ] Check for new dependencies: `go mod download`
- [ ] Build to correct location: `/opt/atmosphere/atmosphere`
- [ ] Update `.env` with new variables (if any)
- [ ] Restart service: `systemctl restart atmosphere`

After update:
- [ ] Service running: `systemctl status atmosphere`
- [ ] API responding: `curl http://localhost:3000/health`
- [ ] Apps still accessible: Test via browser/curl
- [ ] Deployments work: Test deploy an existing app
- [ ] Logs clean: `journalctl -u atmosphere -n 50`

---

## Update Frequency

**Recommended update schedule:**

- **Security updates**: Immediately
- **Feature updates**: Within 1 week of release
- **Patch updates**: Within 2 weeks of release

**Check for updates:**
```bash
cd ~/atmosphere
git fetch origin
git log HEAD..origin/main --oneline
```

**Watch releases:**
- Star the repository on GitHub
- Enable notifications for releases
- Check GitHub releases page: https://github.com/kkassovic/Atmosphere/releases

---

## Automatic Updates (Advanced)

For automated updates, create a cron job:

```bash
# Create update script
sudo nano /opt/atmosphere/auto-update.sh
```

```bash
#!/bin/bash
# Atmosphere auto-update script

LOG="/opt/atmosphere/logs/auto-update.log"
echo "$(date): Starting auto-update" >> "$LOG"

# Pull latest
cd ~/atmosphere
git fetch origin

# Check if update available
if git diff HEAD origin/main --quiet; then
    echo "$(date): Already up to date" >> "$LOG"
    exit 0
fi

# Backup
cp /opt/atmosphere/atmosphere /opt/atmosphere/atmosphere.backup.$(date +%Y%m%d-%H%M%S)
cp /opt/atmosphere/atmosphere.db /opt/atmosphere/atmosphere.db.backup.$(date +%Y%m%d-%H%M%S)

# Update
git pull origin main
cd ~/atmosphere/backend
go build -o /opt/atmosphere/atmosphere ./cmd/atmosphere

# Restart
systemctl restart atmosphere

# Verify
sleep 3
if systemctl is-active --quiet atmosphere; then
    echo "$(date): Update successful" >> "$LOG"
else
    echo "$(date): Update failed, rolling back" >> "$LOG"
    cp /opt/atmosphere/atmosphere.backup.* /opt/atmosphere/atmosphere
    systemctl restart atmosphere
    exit 1
fi
```

```bash
# Make executable
sudo chmod +x /opt/atmosphere/auto-update.sh

# Schedule (daily at 3 AM)
sudo crontab -e
# Add: 0 3 * * * /opt/atmosphere/auto-update.sh
```

**Warning:** Automatic updates can break production. Only use if you:
- Have monitoring in place
- Test updates in staging first
- Have automated rollback procedures

---

## Getting Help

If you encounter issues during updates:

1. **Check logs**: `journalctl -u atmosphere -n 100`
2. **Read release notes**: Check GitHub releases for breaking changes
3. **Ask for help**: Open an issue on GitHub with:
   - Your Atmosphere version
   - Update steps you followed
   - Error messages from logs
   - Output of `systemctl status atmosphere`

---

**Last Updated:** April 12, 2026  
**Atmosphere Version:** 1.0
