#!/bin/bash
#
# atmosphere Update Script
# Updates atmosphere server and CLI binaries from local repository source.
#

set -e

SCRIPT_PATH="$(readlink -f "${BASH_SOURCE[0]}")"
SCRIPT_DIR="$(dirname "$SCRIPT_PATH")"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKEND_DIR="$PROJECT_ROOT/backend"
INSTALL_DIR="/opt/atmosphere"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

if [[ $EUID -ne 0 ]]; then
  echo -e "${RED}Error: This script must be run as root${NC}"
  exit 1
fi

if [ ! -d "$BACKEND_DIR" ]; then
  echo -e "${RED}Error: backend source not found at $BACKEND_DIR${NC}"
  exit 1
fi

echo -e "${YELLOW}Preparing update...${NC}"
mkdir -p "$INSTALL_DIR"

echo -e "${YELLOW}Updating source repository...${NC}"
cd "$PROJECT_ROOT"
git -c safe.directory="$PROJECT_ROOT" fetch origin main
git -c safe.directory="$PROJECT_ROOT" reset --hard origin/main

if ! command -v go &> /dev/null; then
  echo -e "${YELLOW}Installing Go...${NC}"
  GO_VERSION="1.22.1"
  wget -q https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
  rm go${GO_VERSION}.linux-amd64.tar.gz
  export PATH=$PATH:/usr/local/go/bin
  if ! grep -q "/usr/local/go/bin" /etc/profile; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile
  fi
else
  export PATH=$PATH:/usr/local/go/bin
fi

echo -e "${YELLOW}Installing build tools...${NC}"
apt-get update -qq
apt-get install -y -qq build-essential > /dev/null

echo -e "${YELLOW}Backing up binaries...${NC}"
TS="$(date +%Y%m%d-%H%M%S)"
if [ -f "$INSTALL_DIR/atmosphere" ]; then
  cp "$INSTALL_DIR/atmosphere" "$INSTALL_DIR/atmosphere.backup.$TS"
fi
if [ -f "$INSTALL_DIR/atmosphere-cli" ]; then
  cp "$INSTALL_DIR/atmosphere-cli" "$INSTALL_DIR/atmosphere-cli.backup.$TS"
fi

echo -e "${YELLOW}Building latest binaries...${NC}"
cd "$BACKEND_DIR"
export GOFLAGS="-mod=mod"
if [ ! -f go.sum ]; then
  echo -e "${YELLOW}go.sum missing, generating dependency checksums...${NC}"
  go mod tidy
fi
if ! go mod download; then
  echo -e "${YELLOW}go mod download failed, repairing module checksums...${NC}"
  go mod tidy
  go mod download
fi
CGO_ENABLED=1 go build -o "$INSTALL_DIR/atmosphere" ./cmd/atmosphere
CGO_ENABLED=1 go build -o "$INSTALL_DIR/atmosphere-cli" ./cmd/atmosphere-cli
chmod +x "$INSTALL_DIR/atmosphere" "$INSTALL_DIR/atmosphere-cli"
ln -sf "$INSTALL_DIR/atmosphere-cli" /usr/local/bin/atmosphere-cli

echo -e "${YELLOW}Restarting atmosphere service...${NC}"
systemctl restart atmosphere

if systemctl is-active --quiet atmosphere; then
  echo -e "${GREEN}✓ Update successful${NC}"
  echo -e "${GREEN}✓ Server: $INSTALL_DIR/atmosphere${NC}"
  echo -e "${GREEN}✓ CLI: $INSTALL_DIR/atmosphere-cli (linked as atmosphere-cli)${NC}"
else
  echo -e "${RED}Error: atmosphere service failed to start${NC}"
  exit 1
fi
