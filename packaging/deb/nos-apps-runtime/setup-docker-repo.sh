#!/bin/bash
set -euo pipefail

# Setup Docker repository for Debian 13 (trixie)
# This script is idempotent and can be run multiple times safely

DISTRO="debian"
CODENAME="trixie"
DOCKER_REPO_URL="https://download.docker.com/linux/${DISTRO}"
DOCKER_GPG_KEY="/usr/share/keyrings/docker-archive-keyring.gpg"
DOCKER_LIST="/etc/apt/sources.list.d/docker.list"

log() {
    echo "[nos-apps-runtime] $*" >&2
}

# Check if running as root
if [[ $EUID -ne 0 ]]; then
    log "ERROR: This script must be run as root"
    exit 1
fi

# Install prerequisites
log "Installing prerequisites..."
apt-get update -qq
apt-get install -y -qq \
    ca-certificates \
    curl \
    gnupg \
    lsb-release \
    apt-transport-https

# Add Docker's official GPG key
log "Adding Docker GPG key..."
if [[ ! -f "${DOCKER_GPG_KEY}" ]]; then
    mkdir -p "$(dirname "${DOCKER_GPG_KEY}")"
    curl -fsSL "${DOCKER_REPO_URL}/gpg" | gpg --dearmor -o "${DOCKER_GPG_KEY}"
    chmod 644 "${DOCKER_GPG_KEY}"
fi

# Set up the repository
log "Configuring Docker repository..."
echo "deb [arch=$(dpkg --print-architecture) signed-by=${DOCKER_GPG_KEY}] ${DOCKER_REPO_URL} ${CODENAME} stable" > "${DOCKER_LIST}"

# Update package index
log "Updating package index..."
apt-get update -qq

# List available Docker versions (for debugging)
if command -v docker &>/dev/null; then
    log "Current Docker version: $(docker --version)"
else
    log "Docker not yet installed"
fi

log "Docker repository setup complete"
