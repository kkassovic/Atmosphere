# Backup Storage Options - Setup Guide

Atmosphere supports three backup storage backends, each with different advantages:

Important: if you run atmosphere via systemd, the service reads configuration from `/opt/atmosphere/.env`.
If you edit `.env` in your repository checkout (for example `~/atmosphere/.env`), copy it to `/opt/atmosphere/.env` and restart the service.

```bash
sudo cp ~/atmosphere/.env /opt/atmosphere/.env
sudo systemctl restart atmosphere
```

## Option 1: DigitalOcean S3 / AWS S3 (Cloud Storage)

**Best for:** Remote offsite backups, disaster recovery across data centers

### Configuration

```bash
# .env or environment variables
S3_ENDPOINT=https://nyc3.digitaloceanspaces.com
S3_BUCKET=my-backups-bucket
S3_REGION=nyc3
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
S3_PATH_PREFIX=atmosphere-backups
```

### Setup Steps

#### DigitalOcean Spaces
1. Create a Space in DigitalOcean console
2. Generate API token (Settings → API → Spaces)
3. Note the region endpoint:
   - NYC: `https://nyc3.digitaloceanspaces.com`
   - SFO: `https://sfo3.digitaloceanspaces.com`
   - AMS: `https://ams3.digitaloceanspaces.com`

#### AWS S3
1. Create S3 bucket in your region
2. Create IAM user with S3 access
3. Generate access keys
4. Endpoint: `https://s3.region.amazonaws.com` (or custom domain)

#### MinIO (Self-hosted)
1. Deploy MinIO server
2. Create bucket
3. Endpoint: `http://your-minio-server:9000` (or HTTPS)

### Advantages
- ✅ Geographic redundancy
- ✅ Automatic scaling
- ✅ Low cost for storage
- ✅ Works across continents
- ✅ No hardware to manage

### Backup Usage
```bash
curl -X POST http://localhost:3000/api/v1/apps/myapp/backups \
  -H "Content-Type: application/json" \
  -d '{"upload_to_s3": true}'
```

---

## Option 2: Mounted CIFS/SMB Network Share

**Best for:** Local network storage, NAS systems, shared infrastructure

### Configuration

Mount the CIFS/SMB share first, then set the path:

```bash
# .env or environment variables
LOGS_DIR=/mnt/atmosphere-backups
# Don't set S3 variables
```

### Setup Steps

#### Linux - Mount CIFS Share

```bash
# Install cifs-utils
sudo apt-get install cifs-utils

# Create mount point
sudo mkdir -p /mnt/atmosphere-backups

# Mount (temporary)
sudo mount -t cifs //nas-server/backups /mnt/atmosphere-backups \
  -o username=backup_user,password=backup_pass,vers=3.0,uid=1000

# Or mount (permanent) - add to /etc/fstab
//nas-server/backups /mnt/atmosphere-backups cifs \
  credentials=/etc/cifs-credentials,uid=1000,gid=1000,file_mode=0755,dir_mode=0755 0 0

# Create credentials file
sudo nano /etc/cifs-credentials
# Add:
# username=backup_user
# password=backup_pass
sudo chmod 600 /etc/cifs-credentials
```

#### Windows - Mount SMB Share

```powershell
# Map network drive (temporary)
net use Z: \\nas-server\backups /user:backup_user backup_pass

# Or use PowerShell (persistent)
New-SmbGlobalMapping -RemotePath \\nas-server\backups `
  -LocalPath Z: `
  -UserName domain\backup_user `
  -Password "backup_pass" `
  -Persistent $true

# Configure in .env
LOGS_DIR=Z:\atmosphere-backups
```

#### Docker - Mount CIFS in docker-compose.yml

```yaml
services:
  atmosphere:
    volumes:
      - type: bind
        source: /mnt/atmosphere-backups
        target: /opt/atmosphere/logs
```

### Advantages
- ✅ Local network, fast transfers
- ✅ Existing NAS/network infrastructure
- ✅ Hardware control
- ✅ No cloud provider dependencies
- ✅ Suitable for on-premises deployments

### Network Share Examples
- Synology NAS: `\\synology-nas/backups`
- QNAP NAS: `\\qnap-nas/backups`
- Windows Server: `\\file-server/backups`
- FreeNAS: `\\freenas/backups`

---

## Option 3: Mounted USB Drive / External Storage

**Best for:** Local backups, portable storage, off-site physical transport

### Configuration

Mount the USB drive and set the path:

```bash
# .env or environment variables
LOGS_DIR=/mnt/usb-backup
# Don't set S3 variables
```

### Setup Steps

#### Linux - Mount USB Drive

```bash
# List connected drives
lsblk

# Create mount point
sudo mkdir -p /mnt/usb-backup

# Mount USB drive (example: /dev/sdb1)
sudo mount /dev/sdb1 /mnt/usb-backup

# Verify
df -h | grep usb

# Unmount safely
sudo umount /mnt/usb-backup
```

#### Windows - Mount USB Drive

```powershell
# USB drive typically auto-mounts as D:, E:, etc.
# Configure in .env
LOGS_DIR=E:\atmosphere-backups

# Or use diskpart to mount to folder
```

#### Docker - Mount USB in docker-compose.yml

```yaml
services:
  atmosphere:
    volumes:
      - /mnt/usb-backup:/opt/atmosphere/logs
```

### Advantages
- ✅ Completely local and offline
- ✅ Portable - can transport physically
- ✅ No network required
- ✅ Cost-effective
- ✅ Simple setup

### Best Practices
- Use high-speed USB 3.0+ drives
- Test read/write performance before deployment
- Rotate multiple USB drives for off-site storage
- Format with ext4/NTFS with journaling
- Monitor drive health

---

## Comparison Table

| Feature | S3 | CIFS/SMB | USB Drive |
|---------|-----|----------|-----------|
| **Speed** | Network dependent | LAN speed | USB speed |
| **Capacity** | Unlimited | NAS capacity | Drive size |
| **Cost** | Pay per GB | One-time hardware | One-time purchase |
| **Redundancy** | Built-in (multi-region) | NAS RAID | Manual (swap drives) |
| **Offsite** | Geographic redundancy | Network dependent | Physical transport |
| **Setup** | Cloud account | Network mount | USB connection |
| **Reliability** | Provider uptime | NAS uptime | Drive failure risk |
| **Compliance** | SOC2 certified | Self-managed | Self-managed |

---

## Hybrid Setup (Recommended for Production)

Combine multiple backends for maximum resilience:

### Strategy 1: S3 + CIFS (Primary + Backup)
```bash
# Primary: S3 for offsite disaster recovery
LOGS_DIR=/mnt/nas-backups  # For quick restores

# Use CIFS mount
S3_ENDPOINT=...
S3_BUCKET=...
# Create backups to S3 for offsite protection
```

### Strategy 2: S3 + USB (Offsite + Offline)
```bash
# Primary: S3 for regular backups
S3_ENDPOINT=...
S3_BUCKET=...

# Monthly: Sync to USB drive for physical offsite storage
# rsync -av /opt/atmosphere/logs/backups/ /mnt/usb-backup/
```

### Strategy 3: S3 + CIFS + USB (Enterprise)
```bash
# Immediate local backup: CIFS mount
LOGS_DIR=/mnt/nas-backups

# Enable S3 upload: 
S3_ENDPOINT=...
S3_BUCKET=...

# Quarterly: Sync to USB for vault storage
```

---

## Health Check - Verify Storage is Working

Test if your configured backup storage is working:

```bash
# Check backup storage status
curl http://localhost:3000/api/v1/backup-storage/health | jq .
```

For complete health check documentation, see [BACKUP_STORAGE_HEALTH_API.md](BACKUP_STORAGE_HEALTH_API.md).

Success response with S3:
```json
{
  "status": "ok",
  "backend": "s3",
  "s3_endpoint": "https://nyc3.digitaloceanspaces.com",
  "s3_bucket": "my-backups-bucket",
  "message": "S3 connection successful"
}
```

Success response with local storage:
```json
{
  "status": "ok",
  "backend": "local",
  "logs_dir": "/opt/atmosphere/logs",
  "message": "Local storage accessible"
}
```

### Check if storage is mounted/accessible

The previous example already covers this. Additional checks:

```bash
# For CIFS mounts
mount | grep cifs

# For USB drives
lsblk
df -h
```

---

## Troubleshooting

### S3 Connection Issues
```bash
# Test S3 credentials and connectivity
aws s3 ls s3://your-bucket/ --region your-region
```

### CIFS Mount Issues
```bash
# Test connection
smbclient -L //nas-server -U backup_user

# Check mount
mount | grep cifs

# Remount with debugging
mount -t cifs //nas-server/backups /mnt/test \
  -o username=user,password=pass,vers=3.0,debug=1
```

### USB Drive Issues
```bash
# Verify USB is recognized
lsblk
lsusb

# Check file system
sudo fsck /dev/sdb1

# Test write speed
dd if=/dev/zero of=/mnt/usb/test.img bs=1M count=100
```

---

## Automated Backup to USB (Rotation Strategy)

For long-term archival, rotate USB drives:

```bash
#!/bin/bash
# Monthly backup rotation script

BACKUP_SOURCE="/opt/atmosphere/logs/backups"
USB_MOUNT="/mnt/usb-backup"
DATE=$(date +%Y-%m-%d)

# Mount USB
sudo mount /dev/sdb1 $USB_MOUNT

# Sync with delete (update remote)
rsync -av --delete $BACKUP_SOURCE/ $USB_MOUNT/backups-$DATE/

# Verify
du -sh $USB_MOUNT/backups-$DATE

# Unmount safely
sudo umount $USB_MOUNT

echo "Backup completed to USB: $USB_MOUNT/backups-$DATE"
```

---

## Migration Between Storage Backends

### S3 → CIFS
```bash
# 1. Download all backups from S3
aws s3 sync s3://my-bucket/atmosphere-backups /tmp/backups

# 2. Mount CIFS
sudo mount -t cifs //nas-server/backups /mnt/atmosphere-backups

# 3. Copy to CIFS
cp -rv /tmp/backups/* /mnt/atmosphere-backups/

# 4. Update .env and restart
LOGS_DIR=/mnt/atmosphere-backups
```

### CIFS → S3
```bash
# 1. Copy backups to temp location
cp -rv /mnt/nas-backups /tmp/backups

# 2. Configure S3
S3_ENDPOINT=...
S3_BUCKET=...

# 3. Sync to S3
aws s3 sync /tmp/backups s3://my-bucket/atmosphere-backups/

# 4. Verify
aws s3 ls s3://my-bucket/atmosphere-backups --recursive
```

---

## Recommendations by Use Case

### Development/Testing
→ **USB Drive or Local Filesystem**
- Fast, simple setup
- No external dependencies

### Small Business (Single Location)
→ **CIFS to NAS**
- Reliable, manageable
- Can add RAID for redundancy
- All on local network

### Multi-Location / High Availability
→ **S3 with CIFS Backup**
- S3 for disaster recovery
- CIFS for fast local restores
- Best of both worlds

### Enterprise / Compliance
→ **S3 + CIFS + USB Vault**
- S3 for automated offsite
- CIFS for fast access
- USB for offline compliance
- Meets backup 3-2-1 rule (3 copies, 2 media types, 1 offsite)

