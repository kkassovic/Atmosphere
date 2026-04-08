#!/bin/bash
#
# atmosphere Installer
# Installs atmosphere deployment platform on Ubuntu 24.04 LTS
#
# This script:
# - Installs Docker Engine and required tools
# - Sets up Traefik reverse proxy
# - Creates necessary directories and networks
# - Installs atmosphere as a systemd service
# - Configures basic firewall rules
#

set -e

# Get absolute paths FIRST before any directory changes
SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
INSTALL_DIR="/opt/atmosphere"
TRAEFIK_DIR="/opt/traefik"
DOCKER_NETWORK="atmosphere"
TRAEFIK_NETWORK="traefik"

echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   atmosphere Platform Installer       ║${NC}"
echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}Error: This script must be run as root${NC}" 
   exit 1
fi

# Check Ubuntu version
echo -e "${YELLOW}Checking system requirements...${NC}"
if ! grep -q "Ubuntu" /etc/os-release; then
    echo -e "${YELLOW}Warning: This script is designed for Ubuntu. Proceeding anyway...${NC}"
fi

# Update package list
echo -e "${YELLOW}Updating package list...${NC}"
apt-get update -qq

# Install prerequisites
echo -e "${YELLOW}Installing prerequisites...${NC}"
apt-get install -y -qq \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    git \
    jq \
    s3cmd \
    ufw \
    > /dev/null

# Install Docker if not already installed
if ! command -v docker &> /dev/null; then
    echo -e "${YELLOW}Installing Docker Engine...${NC}"
    
    # Add Docker's official GPG key
    install -m 0755 -d /etc/apt/keyrings
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
    chmod a+r /etc/apt/keyrings/docker.gpg
    
    # Set up the repository
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
    
    # Install Docker Engine
    apt-get update -qq
    apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin > /dev/null
    
    # Enable and start Docker
    systemctl enable docker
    systemctl start docker
    
    echo -e "${GREEN}✓ Docker Engine installed${NC}"
else
    echo -e "${GREEN}✓ Docker already installed${NC}"
fi

# Verify Docker Compose plugin
if ! docker compose version &> /dev/null; then
    echo -e "${RED}Error: Docker Compose plugin not found${NC}"
    exit 1
fi

# Create Docker networks
echo -e "${YELLOW}Creating Docker networks...${NC}"
if ! docker network inspect $DOCKER_NETWORK &> /dev/null; then
    docker network create $DOCKER_NETWORK
    echo -e "${GREEN}✓ Created $DOCKER_NETWORK network${NC}"
else
    echo -e "${GREEN}✓ Network $DOCKER_NETWORK already exists${NC}"
fi

if ! docker network inspect $TRAEFIK_NETWORK &> /dev/null; then
    docker network create $TRAEFIK_NETWORK
    echo -e "${GREEN}✓ Created $TRAEFIK_NETWORK network${NC}"
else
    echo -e "${GREEN}✓ Network $TRAEFIK_NETWORK already exists${NC}"
fi

# Create installation directory
echo -e "${YELLOW}Creating directories...${NC}"
mkdir -p $INSTALL_DIR
mkdir -p $INSTALL_DIR/workspaces
mkdir -p $INSTALL_DIR/keys
mkdir -p $INSTALL_DIR/logs
mkdir -p $TRAEFIK_DIR
mkdir -p $TRAEFIK_DIR/acme

# Set permissions on keys directory
chmod 700 $INSTALL_DIR/keys

echo -e "${GREEN}✓ Directories created${NC}"

# Copy or create configuration files
echo -e "${YELLOW}Setting up configuration...${NC}"

# Create .env if it doesn't exist
if [ ! -f "$INSTALL_DIR/.env" ]; then
    cat > "$INSTALL_DIR/.env" << 'EOF'
# atmosphere Configuration
PORT=3000
HOST=0.0.0.0
DATABASE_PATH=/opt/atmosphere/atmosphere.db
WORKSPACES_DIR=/opt/atmosphere/workspaces
KEYS_DIR=/opt/atmosphere/keys
LOGS_DIR=/opt/atmosphere/logs
DOCKER_NETWORK=atmosphere
TRAEFIK_NETWORK=traefik
LETSENCRYPT_EMAIL=admin@example.com
TRAEFIK_DASHBOARD=false
EOF
    echo -e "${GREEN}✓ Created .env configuration${NC}"
    echo -e "${YELLOW}⚠ Please edit $INSTALL_DIR/.env and set LETSENCRYPT_EMAIL${NC}"
else
    echo -e "${GREEN}✓ Configuration already exists${NC}"
fi

# Set up Traefik
echo -e "${YELLOW}Setting up Traefik...${NC}"

# Create Traefik static configuration
cat > "$TRAEFIK_DIR/traefik.yml" << 'EOF'
# Traefik Static Configuration
# API and Dashboard
api:
  dashboard: false
  insecure: false

# Entry Points
entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"
    http:
      tls:
        certResolver: letsencrypt

# Providers
providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: traefik

# Certificate Resolvers
certificatesResolvers:
  letsencrypt:
    acme:
      email: admin@example.com
      storage: /acme/acme.json
      httpChallenge:
        entryPoint: web

# Logs
log:
  level: INFO

accessLog:
  enabled: true
EOF

# Create Traefik docker-compose.yml
cat > "$TRAEFIK_DIR/docker-compose.yml" << 'EOF'
services:
  traefik:
    image: traefik:v2.11
    container_name: traefik
    restart: unless-stopped
    security_opt:
      - no-new-privileges:true
    networks:
      - traefik
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./traefik.yml:/traefik.yml:ro
      - ./acme:/acme
    labels:
      - "traefik.enable=false"

networks:
  traefik:
    external: true
EOF

# Create acme.json with correct permissions
touch $TRAEFIK_DIR/acme/acme.json
chmod 600 $TRAEFIK_DIR/acme/acme.json

# Start Traefik
echo -e "${YELLOW}Starting Traefik...${NC}"
cd $TRAEFIK_DIR
docker compose up -d

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Traefik started successfully${NC}"
else
    echo -e "${RED}Error: Failed to start Traefik${NC}"
    exit 1
fi

# Build atmosphere binary (if source is available)
# Use the PROJECT_ROOT we calculated at the start
BACKEND_DIR=""
if [ -d "$PROJECT_ROOT/backend" ]; then
    BACKEND_DIR="$PROJECT_ROOT/backend"
    echo -e "${YELLOW}Found backend at: $BACKEND_DIR${NC}"
fi

if [ -n "$BACKEND_DIR" ]; then
    echo -e "${YELLOW}Building atmosphere...${NC}"
    
    # Install Go if not present
    if ! command -v go &> /dev/null; then
        echo -e "${YELLOW}Installing Go...${NC}"
        GO_VERSION="1.22.1"
        
        # Download Go
        wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
        if [ $? -ne 0 ]; then
            echo -e "${RED}Error: Failed to download Go${NC}"
            exit 1
        fi
        
        # Remove old Go installation
        rm -rf /usr/local/go
        
        # Extract new Go
        tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
        rm go${GO_VERSION}.linux-amd64.tar.gz
        
        # Add to PATH
        export PATH=$PATH:/usr/local/go/bin
        
        # Add to profile for future sessions
        if ! grep -q "/usr/local/go/bin" /etc/profile; then
            echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
        fi
        
        # Verify Go installation
        if ! /usr/local/go/bin/go version &> /dev/null; then
            echo -e "${RED}Error: Go installation failed${NC}"
            exit 1
        fi
        
        echo -e "${GREEN}✓ Go ${GO_VERSION} installed${NC}"
    else
        echo -e "${GREEN}✓ Go already installed: $(go version)${NC}"
    fi
    
    # Build atmosphere
    echo -e "${YELLOW}Downloading Go dependencies...${NC}"
    cd "$BACKEND_DIR"
    
    # Ensure Go is in PATH
    export PATH=$PATH:/usr/local/go/bin
    
    # Download dependencies and generate go.sum
    go mod tidy
    if [ $? -ne 0 ]; then
        echo -e "${RED}Error: Failed to download Go dependencies${NC}"
        exit 1
    fi
    
    # Build binary
    echo -e "${YELLOW}Compiling atmosphere binary...${NC}"
    go build -o $INSTALL_DIR/atmosphere ./cmd/atmosphere
    BUILD_STATUS=$?
    
    cd - > /dev/null
    
    if [ $BUILD_STATUS -eq 0 ]; then
        # Make binary executable
        chmod +x $INSTALL_DIR/atmosphere
        echo -e "${GREEN}✓ atmosphere built successfully${NC}"
    else
        echo -e "${RED}Error: Failed to build atmosphere${NC}"
        echo -e "${YELLOW}Try running manually:${NC}"
        echo -e "  cd $BACKEND_DIR"
        echo -e "  go build -o $INSTALL_DIR/atmosphere ./cmd/atmosphere"
        exit 1
    fi
else
    echo -e "${YELLOW}⚠ Backend source not found, skipping build${NC}"
    echo -e "${YELLOW}  You'll need to manually copy the atmosphere binary to $INSTALL_DIR${NC}"
fi

# Create systemd service
echo -e "${YELLOW}Creating systemd service...${NC}"
cat > /etc/systemd/system/atmosphere.service << EOF
[Unit]
Description=atmosphere Deployment Platform
After=docker.service
Requires=docker.service

[Service]
Type=simple
User=root
WorkingDirectory=$INSTALL_DIR
EnvironmentFile=$INSTALL_DIR/.env
ExecStart=$INSTALL_DIR/atmosphere
Restart=always
RestartSec=10

# Security settings
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and enable service
systemctl daemon-reload
systemctl enable atmosphere

# Start service if binary exists
if [ -f "$INSTALL_DIR/atmosphere" ]; then
    systemctl start atmosphere
    echo -e "${GREEN}✓ atmosphere service started${NC}"
else
    echo -e "${YELLOW}⚠ atmosphere binary not found, service not started${NC}"
fi

# Configure firewall
echo -e "${YELLOW}Configuring firewall...${NC}"
if command -v ufw &> /dev/null; then
    ufw --force enable
    ufw allow 22/tcp comment 'SSH'
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'
    ufw allow 3000/tcp comment 'atmosphere API'
    echo -e "${GREEN}✓ Firewall configured${NC}"
else
    echo -e "${YELLOW}⚠ UFW not available, skipping firewall configuration${NC}"
fi

# Print summary
echo ""
echo -e "${GREEN}╔═══════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   Installation Complete!              ║${NC}"
echo -e "${GREEN}╚═══════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}atmosphere has been installed successfully!${NC}"
echo ""
echo -e "Installation directory: ${YELLOW}$INSTALL_DIR${NC}"
echo -e "Traefik directory: ${YELLOW}$TRAEFIK_DIR${NC}"
echo ""
echo -e "Configuration file: ${YELLOW}$INSTALL_DIR/.env${NC}"
echo -e "${YELLOW}⚠ Important: Edit the .env file and set LETSENCRYPT_EMAIL${NC}"
echo ""
echo "Service management:"
echo "  Status:  systemctl status atmosphere"
echo "  Start:   systemctl start atmosphere"
echo "  Stop:    systemctl stop atmosphere"
echo "  Restart: systemctl restart atmosphere"
echo "  Logs:    journalctl -u atmosphere -f"
echo ""
echo "Traefik management:"
echo "  cd $TRAEFIK_DIR"
echo "  docker compose logs -f"
echo "  docker compose restart"
echo ""
echo "API will be available at: http://localhost:3000"
echo ""
echo -e "${GREEN}Next steps:${NC}"
echo "1. Edit $INSTALL_DIR/.env and configure your settings"
echo "2. Restart atmosphere: systemctl restart atmosphere"
echo "3. Start deploying apps!"
echo ""
echo -e "${YELLOW}Documentation: https://github.com/kkassovic/atmosphere${NC}"
echo ""
