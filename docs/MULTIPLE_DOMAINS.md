# Multiple Domains per App

Atmosphere supports configuring **multiple HTTPS domains** for a single application. This allows you to route traffic from multiple domain names to the same app container, with automatic SSL certificate provisioning for all domains.

## Table of Contents

1. [Overview](#overview)
2. [How It Works](#how-it-works)
3. [Configuration Examples](#configuration-examples)
4. [Use Cases](#use-cases)
5. [DNS Setup](#dns-setup)
6. [SSL Certificates](#ssl-certificates)
7. [Troubleshooting](#troubleshooting)

---

## Overview

### What Changed

**Before (single domain):**
```json
{
  "domain": "app.example.com"
}
```

**Now (multiple domains):**
```json
{
  "domains": ["app.example.com", "www.app.com", "app.another-domain.org"]
}
```

### Benefits

- **SEO**: Support both www and non-www versions of your domain
- **Branding**: Use multiple branded domains for the same app
- **Aliases**: Create domain aliases without duplicating deployments
- **Migration**: Gradually transition from old to new domain names
- **Multi-regional**: Use country-specific domains (.com, .co.uk, .de) all pointing to one app

---

## How It Works

Atmosphere generates Traefik routing rules using OR logic for multiple domains:

**Single domain:**
```
Host(`app.example.com`)
```

**Multiple domains:**
```
Host(`app.example.com`) || Host(`www.app.com`) || Host(`app.another.org`)
```

All domains route to the same container. Traefik automatically:
- Obtains SSL certificates from Let's Encrypt for **each domain**
- Manages certificate renewals independently
- Redirects HTTP to HTTPS for all domains

---

## Configuration Examples

### Creating an App with Multiple Domains

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domains": [
      "myapp.example.com",
      "www.myapp.example.com",
      "app.another-domain.org"
    ],
    "port": 8080
  }'
```

### Updating Domains on Existing App

**Add more domains:**
```bash
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "domains": [
      "myapp.example.com",
      "www.myapp.example.com",
      "app.another-domain.org",
      "newdomain.com"
    ]
  }'
```

**Then redeploy to apply changes:**
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Single Domain (Backward Compatible)

You can still use just one domain:

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "simple-app",
    "deployment_type": "manual",
    "build_type": "compose",
    "domains": ["simple.example.com"]
  }'
```

### No Domain (Internal/Local Only)

For internal apps that don't need public domains:

```bash
curl -X POST http://localhost:3000/api/v1/apps \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internal-app",
    "deployment_type": "manual",
    "build_type": "dockerfile",
    "domains": [],
    "port": 3000
  }'
```

App will be accessible via Docker network but not through Traefik.

---

## Use Cases

### 1. www and non-www Versions

```json
{
  "domains": [
    "example.com",
    "www.example.com"
  ]
}
```

Both URLs work and get SSL certificates. Your application can redirect one to the other if desired.

### 2. Multiple Branded Domains

```json
{
  "domains": [
    "myproduct.com",
    "myproduct.io",
    "myproduct.app"
  ]
}
```

Use multiple TLDs for the same product.

### 3. Country-Specific Domains

```json
{
  "domains": [
    "myapp.com",
    "myapp.co.uk",
    "myapp.de",
    "myapp.fr"
  ]
}
```

Your application can detect the domain and serve localized content.

### 4. Domain Migration

```json
{
  "domains": [
    "old-brand.com",
    "new-brand.com"
  ]
}
```

Keep old domain working while transitioning to new branding. Your app can redirect old to new.

### 5. Subdomain Variations

```json
{
  "domains": [
    "app.example.com",
    "dashboard.example.com",
    "portal.example.com"
  ]
}
```

Different subdomains for the same application.

---

## DNS Setup

For each domain you configure, create an A record pointing to your server:

```
A    myapp.example.com        →    YOUR_SERVER_IP
A    www.myapp.example.com    →    YOUR_SERVER_IP
A    app.another-domain.org   →    YOUR_SERVER_IP
```

### Using Wildcard DNS

If you control the entire domain, use a wildcard:

```
A    *.example.com    →    YOUR_SERVER_IP
```

This covers all subdomains without individual A records.

### Verify DNS

Before deploying, verify DNS propagation:

```bash
# Check each domain
dig +short myapp.example.com
dig +short www.myapp.example.com
dig +short app.another-domain.org
```

All should return your server IP.

---

## SSL Certificates

### Automatic Certificate Provisioning

When you deploy an app with multiple domains, Traefik automatically:

1. Requests SSL certificates from Let's Encrypt for **each domain**
2. Stores certificates in `/opt/traefik/acme/acme.json`
3. Renews certificates before expiration

### Certificate Requirements

For Let's Encrypt to issue certificates:

- ✅ Domain must resolve to your server IP (DNS A record)
- ✅ Port 80 must be open and accessible from internet
- ✅ Port 443 must be open
- ✅ Valid email configured in Traefik (`LETSENCRYPT_EMAIL`)

### Monitoring Certificates

View Traefik logs to see certificate provisioning:

```bash
cd /opt/traefik
docker compose logs -f
```

Look for:
```
[acme] Requesting certificate for myapp.example.com
[acme] Server responded with a certificate for myapp.example.com
```

### Certificate Limits

Let's Encrypt has rate limits:
- **50 certificates per domain per week**
- With 3 domains, you can deploy ~16 times per week
- Staging certificates (for testing) have higher limits

**Best practice:** Test with staging environment first if making many changes.

---

## Troubleshooting

### Issue: Certificate Not Generated

**Symptom:** HTTPS not working, browser shows "Not Secure"

**Solutions:**

1. **Check DNS:**
   ```bash
   dig +short yourdomain.com
   ```
   Should return your server IP.

2. **Check Traefik logs:**
   ```bash
   cd /opt/traefik
   docker compose logs | grep -i acme
   ```

3. **Verify ports open:**
   ```bash
   sudo ufw status
   ```
   Ports 80 and 443 should be open.

4. **Check domain accessibility:**
   ```bash
   curl http://yourdomain.com
   ```
   Should connect (even if returns 404).

### Issue: Some Domains Work, Others Don't

**Cause:** DNS not properly configured for all domains

**Solution:** Verify each domain resolves:
```bash
for domain in myapp.com www.myapp.com app.other.com; do
  echo "$domain: $(dig +short $domain)"
done
```

### Issue: Domain Updates Not Applied

**Cause:** Forgot to redeploy after updating domains

**Solution:** Always redeploy after changing domains:
```bash
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

### Issue: Rate Limited by Let's Encrypt

**Symptom:** Error: "too many certificates already issued"

**Solutions:**

1. **Wait:** Rate limit resets after 7 days
2. **Use staging:** For testing, configure Traefik to use Let's Encrypt staging
3. **Reduce deployments:** Don't deploy more than 50 times per week per domain

---

## Environment Variables

Your application receives domain information through environment variables:

```bash
DOMAIN=myapp.example.com        # First domain (for backward compatibility)
DOMAINS=myapp.example.com,www.myapp.com,app.other.org  # All domains, comma-separated
```

### Using in Your App

**Node.js:**
```javascript
const primaryDomain = process.env.DOMAIN;
const allDomains = process.env.DOMAINS.split(',');

// Redirect www to non-www
app.use((req, res, next) => {
  if (req.hostname === 'www.myapp.com') {
    return res.redirect(301, `https://myapp.com${req.url}`);
  }
  next();
});
```

**Python:**
```python
import os

primary_domain = os.environ.get('DOMAIN')
all_domains = os.environ.get('DOMAINS', '').split(',')

# Serve different content based on domain
if request.host in all_domains:
    # handle request
```

**Go:**
```go
domain := os.Getenv("DOMAIN")
domains := strings.Split(os.Getenv("DOMAINS"), ",")

// Check if request is from valid domain
func isValidDomain(host string) bool {
    for _, d := range domains {
        if d == host {
            return true
        }
    }
    return false
}
```

---

## Migration from Single Domain

If you have existing apps using the old single-domain configuration:

### Automatic Migration

Atmosphere automatically migrates existing apps:
- Old `domain` field is converted to `domains` array with one element
- Existing apps continue working without changes
- You can update to multiple domains anytime

### Manual Update

To add more domains to an existing app:

```bash
# 1. Get current configuration
curl http://localhost:3000/api/v1/apps/my-app

# 2. Update with multiple domains
curl -X PUT http://localhost:3000/api/v1/apps/my-app \
  -H "Content-Type: application/json" \
  -d '{
    "domains": [
      "existing-domain.com",
      "new-domain.com",
      "another-domain.org"
    ]
  }'

# 3. Redeploy
curl -X POST http://localhost:3000/api/v1/apps/my-app/deploy
```

---

## Best Practices

1. **Order matters:** Put your primary domain first in the array
   - It's used for the `DOMAIN` environment variable
   - Appears first in logs and admin interfaces

2. **Verify DNS before deploying:** Check all domains resolve to your server

3. **Plan for SSL rate limits:** Don't deploy too frequently with many domains

4. **Use www redirect:** Choose either www or non-www as canonical, redirect the other

5. **Document domain purpose:** Keep notes on why each domain is configured

6. **Monitor certificates:** Check Traefik logs after deploying new domains

7. **Test thoroughly:** After adding domains, test each one works correctly

---

## API Reference

### domains Field

**Type:** `array` of `string`

**Required:** No (defaults to empty array)

**Example:**
```json
{
  "domains": [
    "app.example.com",
    "www.app.example.com"
  ]
}
```

**Validation:**
- Each domain must be valid format (RFC 1034/1035)
- Lowercase alphanumeric, hyphens, dots only
- No spaces or special characters
- Maximum 253 characters per domain

### Response Format

**GET /api/v1/apps/{name}:**
```json
{
  "id": 1,
  "name": "my-app",
  "domains": [
    "app.example.com",
    "www.app.example.com",
    "app.another.org"
  ],
  ...
}
```

**Empty domains:**
```json
{
  "domains": []
}
```

---

## Summary

Multiple domains per app provides flexibility for:
- ✅ Supporting www and non-www versions
- ✅ Using multiple branded domains
- ✅ Country-specific domains
- ✅ Domain migrations
- ✅ Subdomain variations

All domains get automatic HTTPS via Let's Encrypt and route to the same application container.

For more information, see:
- [Deployment Guide](DEPLOYMENT_GUIDE.md)
- [App Management Guide](APP_MANAGEMENT.md)
- [Traefik Guide](TRAEFIK_GUIDE.md)
