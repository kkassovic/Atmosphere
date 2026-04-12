# Traefik Management Guide

This guide covers managing Traefik, the reverse proxy and SSL/TLS certificate manager for Atmosphere. Learn how to start, stop, monitor, configure, and troubleshoot Traefik.

## Table of Contents

1. [Overview](#overview)
2. [Starting & Stopping Traefik](#starting--stopping-traefik)
3. [Viewing Logs](#viewing-logs)
4. [Configuration](#configuration)
5. [Let's Encrypt & SSL Certificates](#lets-encrypt--ssl-certificates)
6. [Monitoring & Status](#monitoring--status)
7. [Troubleshooting](#troubleshooting)
8. [Updating Traefik](#updating-traefik)
9. [Advanced Configuration](#advanced-configuration)
10. [Quick Reference](#quick-reference)

---

## Overview

### What is Traefik?

Traefik is a modern HTTP reverse proxy and load balancer that:
- Routes incoming requests to the correct application
- Automatically obtains and renews SSL certificates from Let's Encrypt
- Redirects HTTP to HTTPS
- Watches Docker containers and updates routing dynamically

### Architecture

```
Internet
   ↓
   ↓ (Port 80/443)
   ↓
Traefik Container
   ↓
   ├──→ App 1 (app1.example.com)
   ├──→ App 2 (app2.example.com)
   └──→ App 3 (app3.example.com)
```

### File Locations

```
/opt/traefik/
├── docker-compose.yml    # Traefik container definition
├── traefik.yml           # Traefik configuration
└── acme/
    └── acme.json         # Let's Encrypt certificates (sensitive!)
```

---

## Starting & Stopping Traefik

### Check Traefik Status

```bash
docker ps | grep traefik
```

**Expected output:**
```
abc123...  traefik  Up 5 days  0.0.0.0:80->80/tcp, 0.0.0.0:443->443/tcp
```

Or check with docker compose:

```bash
cd /opt/traefik
docker compose ps
```

### Start Traefik

```bash
cd /opt/traefik
docker compose up -d
```

**Output:**
```
[+] Running 1/1
 ✔ Container traefik  Started
```

### Stop Traefik

⚠️ **Warning:** Stopping Traefik will make all apps inaccessible!

```bash
cd /opt/traefik
docker compose stop
```

### Restart Traefik

Restart after configuration changes:

```bash
cd /opt/traefik
docker compose restart
```

Or a full stop/start cycle:

```bash
cd /opt/traefik
docker compose down
docker compose up -d
```

### Auto-Start on Boot

Traefik is configured with `restart: unless-stopped` in docker-compose.yml, so it automatically starts on system boot.

Verify this setting:

```bash
cd /opt/traefik
grep restart docker-compose.yml
```

Should show: `restart: unless-stopped`

---

## Viewing Logs

### Real-Time Logs

Follow logs as they happen:

```bash
cd /opt/traefik
docker compose logs -f
```

Or directly:

```bash
docker logs -f traefik
```

Press `Ctrl+C` to exit.

### Recent Logs

View last 100 lines:

```bash
cd /opt/traefik
docker compose logs --tail 100
```

View last 50 lines:

```bash
docker logs --tail 50 traefik
```

### Logs Since Specific Time

Last hour:

```bash
docker logs --since 1h traefik
```

Last 30 minutes:

```bash
docker logs --since 30m traefik
```

Since specific time:

```bash
docker logs --since "2026-04-12T10:00:00" traefik
```

### Filter Logs

**Certificate-related logs:**

```bash
docker logs traefik 2>&1 | grep -i certif
docker logs traefik 2>&1 | grep -i acme
```

**Error logs only:**

```bash
docker logs traefik 2>&1 | grep -i error
docker logs traefik 2>&1 | grep -i "level=error"
```

**Specific domain:**

```bash
docker logs traefik 2>&1 | grep "app.example.com"
```

**Access logs:**

```bash
docker logs traefik 2>&1 | grep "GET\|POST\|PUT\|DELETE"
```

### Save Logs to File

```bash
docker logs traefik > /tmp/traefik-logs.txt 2>&1
```

With timestamp:

```bash
docker logs traefik > /tmp/traefik-logs-$(date +%Y%m%d-%H%M%S).txt 2>&1
```

---

## Configuration

### View Current Configuration

```bash
cat /opt/traefik/traefik.yml
```

### Configuration Structure

The `traefik.yml` file contains:

```yaml
# API and Dashboard
api:
  dashboard: false      # Dashboard disabled by default (security)
  insecure: false

# Entry Points (ports)
entryPoints:
  web:                  # Port 80 (HTTP)
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:           # Port 443 (HTTPS)
    address: ":443"
    http:
      tls:
        certResolver: letsencrypt

# Docker Provider
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false    # Only expose containers with labels
    network: traefik
    watch: true               # Auto-detect new containers

# Let's Encrypt
certificatesResolvers:
  letsencrypt:
    acme:
      email: your-email@example.com
      storage: /acme/acme.json
      httpChallenge:
        entryPoint: web

# Logging
log:
  level: INFO              # DEBUG, INFO, WARN, ERROR

accessLog: {}             # Enable access logging
```

### Update Email for Let's Encrypt

**Important:** Use a real email address for Let's Encrypt notifications.

```bash
# Edit configuration
sudo nano /opt/traefik/traefik.yml

# Find and update the email line:
#     email: your-email@example.com

# Save and restart Traefik
cd /opt/traefik
docker compose restart
```

### Change Log Level

For more detailed logs (debugging):

```bash
sudo nano /opt/traefik/traefik.yml

# Change:
#   level: INFO
# To:
#   level: DEBUG

# Restart
cd /opt/traefik
docker compose restart
```

**Log levels:**
- `DEBUG` - Very verbose (use for troubleshooting)
- `INFO` - Normal operations (default)
- `WARN` - Warnings only
- `ERROR` - Errors only

### Enable Traefik Dashboard

⚠️ **Security Warning:** The dashboard exposes routing information. Only enable on trusted networks.

Edit `traefik.yml`:

```yaml
api:
  dashboard: true
  insecure: true    # Enables dashboard on port 8080 (no auth!)
```

Then access at: `http://YOUR_SERVER_IP:8080/dashboard/`

**Production Setup:** Use secure dashboard with authentication:

```yaml
api:
  dashboard: true
  insecure: false
```

And add labels to expose it with SSL:

```yaml
# In docker-compose.yml, add to traefik service:
labels:
  - "traefik.enable=true"
  - "traefik.http.routers.dashboard.rule=Host(`traefik.example.com`)"
  - "traefik.http.routers.dashboard.service=api@internal"
  - "traefik.http.routers.dashboard.entrypoints=websecure"
  - "traefik.http.routers.dashboard.tls.certresolver=letsencrypt"
  - "traefik.http.routers.dashboard.middlewares=auth"
  - "traefik.http.middlewares.auth.basicauth.users=admin:$$apr1$$..." # Generated password
```

Generate password:

```bash
echo $(htpasswd -nb admin your-password) | sed -e s/\\$/\\$\\$/g
```

### Disable Access Logs

To reduce log verbosity, disable access logs:

```bash
sudo nano /opt/traefik/traefik.yml

# Comment out or remove:
# accessLog: {}

# Restart
cd /opt/traefik
docker compose restart
```

---

## Let's Encrypt & SSL Certificates

### How It Works

1. App is deployed with domain label
2. Traefik detects the new container
3. Traefik requests SSL certificate from Let's Encrypt
4. Let's Encrypt verifies domain ownership (HTTP challenge on port 80)
5. Certificate is issued and stored in `/opt/traefik/acme/acme.json`
6. Certificate is automatically renewed before expiration (90 days)

### View Certificates

⚠️ **Sensitive file:** Contains private keys. Handle with care!

```bash
sudo cat /opt/traefik/acme/acme.json | jq
```

List domains with certificates:

```bash
sudo cat /opt/traefik/acme/acme.json | jq -r '.letsencrypt.Certificates[].domain.main'
```

Check certificate expiration:

```bash
sudo cat /opt/traefik/acme/acme.json | jq '.letsencrypt.Certificates[] | {domain: .domain.main, expires: .certificate}'
```

### Certificate Location

- **File:** `/opt/traefik/acme/acme.json`
- **Permissions:** `600` (read/write for owner only)
- **Format:** JSON containing certificates and private keys
- **Backup:** Essential to backup this file!

### Check Certificate Validity

Test SSL certificate from outside:

```bash
openssl s_client -connect app.example.com:443 -servername app.example.com < /dev/null 2>&1 | openssl x509 -noout -dates
```

Or use an online tool:
- https://www.ssllabs.com/ssltest/
- https://www.digicert.com/help/

### Force Certificate Renewal

Certificates are automatically renewed 30 days before expiration. To force renewal:

```bash
# 1. Stop Traefik
cd /opt/traefik
docker compose down

# 2. Backup and remove acme.json
sudo cp acme/acme.json acme/acme.json.backup
sudo rm acme/acme.json

# 3. Start Traefik (will request new certificates)
docker compose up -d

# 4. Watch logs for certificate requests
docker logs -f traefik
```

### Certificate Troubleshooting

**Certificate not issued:**

Check logs for errors:

```bash
docker logs traefik 2>&1 | grep -i acme
docker logs traefik 2>&1 | grep -i certificate
```

**Common issues:**
1. Port 80 not accessible (required for HTTP challenge)
2. Domain doesn't point to server
3. Invalid email in configuration
4. Rate limiting (see below)

### Let's Encrypt Rate Limits

Let's Encrypt has rate limits:
- **50 certificates per domain per week**
- **5 duplicate certificates per week**
- **Failed validation limit: 5 failures per hour**

If you hit rate limits:
- Wait for the limit to reset
- Use Let's Encrypt staging server for testing

**Use staging for testing:**

```yaml
certificatesResolvers:
  letsencrypt:
    acme:
      email: your-email@example.com
      storage: /acme/acme.json
      caServer: "https://acme-staging-v02.api.letsencrypt.org/directory"  # Add this
      httpChallenge:
        entryPoint: web
```

Staging certificates won't be trusted by browsers but allow unlimited testing.

### Backup Certificates

**Important:** Backup `acme.json` regularly!

```bash
# Manual backup
sudo cp /opt/traefik/acme/acme.json /opt/traefik/acme/acme.json.backup-$(date +%Y%m%d)

# Automated daily backup
echo "0 2 * * * root cp /opt/traefik/acme/acme.json /opt/traefik/acme/acme.json.backup-\$(date +\%Y\%m\%d)" | sudo tee -a /etc/crontab
```

### Restore Certificates

```bash
cd /opt/traefik
docker compose down
sudo cp acme/acme.json.backup acme/acme.json
sudo chmod 600 acme/acme.json
docker compose up -d
```

---

## Monitoring & Status

### Check Traefik Container Health

```bash
docker ps | grep traefik
```

Detailed status:

```bash
docker inspect traefik | jq '.[0].State'
```

### Check Network Connectivity

Verify Traefik is listening on ports:

```bash
sudo netstat -tlnp | grep -E ':(80|443)'
```

**Expected:**
```
tcp6  0  0 :::80   :::*  LISTEN  12345/docker-proxy
tcp6  0  0 :::443  :::*  LISTEN  12345/docker-proxy
```

### Check Connected Containers

List containers on Traefik network:

```bash
docker network inspect traefik | jq -r '.[0].Containers[].Name'
```

### Monitor Resource Usage

CPU and memory usage:

```bash
docker stats traefik --no-stream
```

Continuous monitoring:

```bash
docker stats traefik
```

### Check Routing Configuration

View current routes (requires dashboard enabled):

```bash
curl http://localhost:8080/api/http/routers | jq
```

---

## Troubleshooting

### Problem: Apps Not Accessible

**Check Traefik is running:**

```bash
docker ps | grep traefik
```

If not running:

```bash
cd /opt/traefik
docker compose up -d
```

**Check Traefik logs for errors:**

```bash
docker logs --tail 50 traefik
```

**Verify app has correct labels:**

```bash
docker inspect atmosphere-my-app | jq '.[0].Config.Labels' | grep traefik
```

Should show:
- `traefik.enable=true`
- `traefik.http.routers.*` labels
- Domain configuration

**Test direct container access:**

```bash
# Get container IP
docker inspect atmosphere-my-app | jq -r '.[0].NetworkSettings.Networks.traefik.IPAddress'

# Try accessing directly
curl http://<container-ip>:3000
```

### Problem: SSL Certificate Not Generated

**Check domain DNS:**

```bash
dig app.example.com +short
nslookup app.example.com
```

Must return your server's IP address.

**Check port 80 is accessible:**

```bash
curl http://YOUR_SERVER_IP
```

Should redirect to HTTPS or show Traefik 404 (not connection refused).

**Check firewall:**

```bash
sudo ufw status
```

Ensure ports 80 and 443 are allowed:

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
```

**Check Let's Encrypt logs:**

```bash
docker logs traefik 2>&1 | grep -i acme
docker logs traefik 2>&1 | grep -i "Unable to obtain"
```

**Common errors:**

- "Forbidden domain" → Invalid email in config
- "Connection refused" → Port 80 blocked
- "No such host" → DNS not configured
- "Rate limit" → Too many requests, use staging or wait

**Verify email is valid:**

```bash
grep email /opt/traefik/traefik.yml
```

Change if needed and restart:

```bash
sudo nano /opt/traefik/traefik.yml
cd /opt/traefik
docker compose restart
```

### Problem: HTTP Redirects Not Working

**Check entrypoint configuration:**

```bash
grep -A 10 "entryPoints:" /opt/traefik/traefik.yml
```

Should have HTTP → HTTPS redirect:

```yaml
web:
  address: ":80"
  http:
    redirections:
      entryPoint:
        to: websecure
        scheme: https
```

### Problem: Traefik Won't Start

**Check logs:**

```bash
cd /opt/traefik
docker compose logs
```

**Common causes:**

1. **Port already in use:**

```bash
sudo netstat -tlnp | grep -E ':(80|443)'
```

Kill process using the port or change Traefik ports.

2. **Invalid configuration:**

```bash
# Validate YAML syntax
python3 -c "import yaml; yaml.safe_load(open('/opt/traefik/traefik.yml'))"
```

3. **Permission issues on acme.json:**

```bash
sudo chmod 600 /opt/traefik/acme/acme.json
```

4. **Docker socket not accessible:**

```bash
ls -l /var/run/docker.sock
```

Should be readable.

### Problem: Multiple Domains Same App

If you want multiple domains for one app, add additional router labels:

```yaml
labels:
  # First domain
  - "traefik.http.routers.myapp.rule=Host(`app.example.com`)"
  - "traefik.http.routers.myapp.entrypoints=websecure"
  - "traefik.http.routers.myapp.tls.certresolver=letsencrypt"
  
  # Second domain
  - "traefik.http.routers.myapp-alt.rule=Host(`another.example.com`)"
  - "traefik.http.routers.myapp-alt.entrypoints=websecure"
  - "traefik.http.routers.myapp-alt.tls.certresolver=letsencrypt"
  
  # Service
  - "traefik.http.services.myapp.loadbalancer.server.port=3000"
```

Or use OR condition:

```yaml
- "traefik.http.routers.myapp.rule=Host(`app.example.com`) || Host(`another.example.com`)"
```

### Problem: High Memory/CPU Usage

**Check stats:**

```bash
docker stats traefik --no-stream
```

**Reduce log verbosity:**

```bash
# Edit traefik.yml
log:
  level: WARN  # Instead of DEBUG or INFO

# Remove access logs
# accessLog: {}  # Comment out
```

**Check for routing loops or misconfigurations:**

```bash
docker logs traefik 2>&1 | grep -i "loop\|circular"
```

### Problem: Certificate Expires Unexpectedly

Traefik automatically renews certificates 30 days before expiration.

**Check Traefik has been running continuously:**

```bash
docker ps | grep traefik
```

**Check logs for renewal attempts:**

```bash
docker logs traefik 2>&1 | grep -i "renew"
```

**Ensure Traefik restarts automatically:**

```bash
docker inspect traefik | jq '.[0].HostConfig.RestartPolicy'
```

Should show: `"Name": "unless-stopped"`

---

## Updating Traefik

### Check Current Version

```bash
docker exec traefik traefik version
```

Or from image:

```bash
docker inspect traefik | jq -r '.[0].Config.Image'
```

### Update to Latest Version

**1. Backup configuration and certificates:**

```bash
cd /opt/traefik
sudo cp -r acme acme.backup-$(date +%Y%m%d)
sudo cp traefik.yml traefik.yml.backup
sudo cp docker-compose.yml docker-compose.yml.backup
```

**2. Edit docker-compose.yml to update version:**

```bash
sudo nano docker-compose.yml

# Change:
#   image: traefik:v2.11
# To:
#   image: traefik:v3.0  # Or latest version
```

**3. Pull new image and restart:**

```bash
cd /opt/traefik
docker compose pull
docker compose down
docker compose up -d
```

**4. Verify new version:**

```bash
docker exec traefik traefik version
```

**5. Check logs for errors:**

```bash
docker logs -f traefik
```

### Rollback to Previous Version

If update causes issues:

```bash
cd /opt/traefik
docker compose down

# Restore backup
sudo cp docker-compose.yml.backup docker-compose.yml
sudo cp traefik.yml.backup traefik.yml

# Start with old version
docker compose up -d
```

### Version Compatibility

- Traefik v2.x → v3.x may require configuration changes
- Check [Traefik migration guide](https://doc.traefik.io/traefik/migration/v2-to-v3/)
- Always backup before major version upgrades

---

## Advanced Configuration

### Custom Middlewares

Add rate limiting, authentication, or headers.

**Example: Add security headers**

In `docker-compose.yml`, add labels:

```yaml
labels:
  - "traefik.enable=true"
  # ... existing labels ...
  
  # Custom middleware for security headers
  - "traefik.http.middlewares.security-headers.headers.customResponseHeaders.X-Frame-Options=SAMEORIGIN"
  - "traefik.http.middlewares.security-headers.headers.customResponseHeaders.X-Content-Type-Options=nosniff"
  - "traefik.http.routers.myapp.middlewares=security-headers"
```

**Example: IP Whitelist**

```yaml
labels:
  - "traefik.http.middlewares.ipwhitelist.ipwhitelist.sourcerange=1.2.3.4/32,5.6.7.0/24"
  - "traefik.http.routers.admin.middlewares=ipwhitelist"
```

### TCP/UDP Routing

Traefik can also route TCP/UDP traffic (databases, etc.)

Add to `traefik.yml`:

```yaml
entryPoints:
  postgres:
    address: ":5432"

tcp:
  routers:
    postgres-router:
      rule: "HostSNI(`*`)"
      service: postgres-service
      entryPoints:
        - postgres
  services:
    postgres-service:
      loadBalancer:
        servers:
          - address: "postgres-container:5432"
```

### Multiple Certificate Resolvers

For different validation methods:

```yaml
certificatesResolvers:
  letsencrypt-http:
    acme:
      email: admin@example.com
      storage: /acme/http.json
      httpChallenge:
        entryPoint: web
  
  letsencrypt-dns:
    acme:
      email: admin@example.com
      storage: /acme/dns.json
      dnsChallenge:
        provider: cloudflare
        resolvers:
          - "1.1.1.1:53"
```

Then use different resolvers for different domains.

### File Provider (Static Routes)

Add routes that don't come from Docker containers:

Create `/opt/traefik/dynamic.yml`:

```yaml
http:
  routers:
    external-api:
      rule: "Host(`api.example.com`)"
      service: external-api-service
      entryPoints:
        - websecure
      tls:
        certResolver: letsencrypt
  
  services:
    external-api-service:
      loadBalancer:
        servers:
          - url: "http://192.168.1.100:8080"
```

Mount in docker-compose.yml:

```yaml
volumes:
  - ./dynamic.yml:/dynamic.yml:ro
```

Add to traefik.yml:

```yaml
providers:
  docker:
    # ... existing config ...
  file:
    filename: /dynamic.yml
    watch: true
```

---

## Quick Reference

### Essential Commands

```bash
# Status
docker ps | grep traefik
cd /opt/traefik && docker compose ps

# Start/Stop/Restart
cd /opt/traefik
docker compose up -d      # Start
docker compose stop       # Stop
docker compose restart    # Restart
docker compose down       # Stop and remove
docker compose up -d      # Recreate and start

# Logs
docker logs -f traefik                      # Follow logs
docker logs --tail 100 traefik              # Last 100 lines
docker logs traefik 2>&1 | grep -i error    # Errors only
docker logs traefik 2>&1 | grep -i acme     # Certificate logs

# Configuration
cat /opt/traefik/traefik.yml                # View config
sudo nano /opt/traefik/traefik.yml          # Edit config
cd /opt/traefik && docker compose restart   # Apply changes

# Certificates
sudo cat /opt/traefik/acme/acme.json | jq                           # View all
sudo cat /opt/traefik/acme/acme.json | jq -r '.letsencrypt.Certificates[].domain.main'  # List domains

# Troubleshooting
docker logs traefik 2>&1 | grep -i error                  # Check errors
dig app.example.com +short                                # Check DNS
curl http://YOUR_SERVER_IP                                # Check port 80
curl -I https://app.example.com                           # Test SSL
docker network inspect traefik | jq -r '.[0].Containers'  # Connected containers
```

### File Locations

| File | Purpose | Backup? |
|------|---------|---------|
| `/opt/traefik/traefik.yml` | Main configuration | Yes |
| `/opt/traefik/docker-compose.yml` | Container definition | Yes |
| `/opt/traefik/acme/acme.json` | SSL certificates | **Critical!** |

### Port Reference

| Port | Protocol | Purpose |
|------|----------|---------|
| 80 | HTTP | HTTP traffic, Let's Encrypt challenges |
| 443 | HTTPS | Secure traffic |
| 8080 | HTTP | Dashboard (if enabled) |

### Log Levels

| Level | Use Case |
|-------|----------|
| `DEBUG` | Troubleshooting, very verbose |
| `INFO` | Normal operations (default) |
| `WARN` | Warnings only |
| `ERROR` | Errors only |

### Certificate Lifecycle

| Stage | Timing | Action |
|-------|--------|--------|
| Initial request | App first deployed | Automatic |
| Renewal check | Daily | Automatic |
| Renewal | 30 days before expiry | Automatic |
| Certificate expires | 90 days after issue | Renew before this! |

---

## Best Practices

1. **Backup `acme.json` regularly** - Losing this means re-requesting all certificates
2. **Use a real email** - Let's Encrypt sends expiration warnings
3. **Monitor logs** - Check for certificate errors or routing issues
4. **Keep Traefik updated** - Security patches and new features
5. **Use staging for testing** - Avoid rate limits when testing new configs
6. **Set resource limits** - Prevent Traefik from consuming too many resources
7. **Enable access logs only when needed** - They can grow large
8. **Document custom configurations** - Note any advanced setups
9. **Test SSL regularly** - Use SSL Labs or similar tools
10. **Keep docker-compose.yml in version control** - Track configuration changes

---

## Additional Resources

- **Official Documentation:** https://doc.traefik.io/traefik/
- **Let's Encrypt Documentation:** https://letsencrypt.org/docs/
- **Traefik Community Forum:** https://community.traefik.io/
- **SSL Labs Test:** https://www.ssllabs.com/ssltest/
- **Let's Encrypt Rate Limits:** https://letsencrypt.org/docs/rate-limits/
