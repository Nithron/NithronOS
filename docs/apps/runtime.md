# nOS Apps Runtime

## Overview

The nOS Apps Runtime provides a secure, Docker-based application platform with built-in snapshot support and lifecycle management. It enables one-click deployment of containerized applications with proper isolation and resource management.

## Architecture

### Components

1. **Docker Engine**: Core container runtime with hardened configuration
2. **docker-compose v2**: Application orchestration via Compose files
3. **Btrfs-aware snapshots**: Atomic snapshots for app data with rsync fallback
4. **Systemd integration**: Templated units for app lifecycle management
5. **CLI helpers**: Tools for nosd integration and container operations

### Directory Structure

```
/srv/apps/                       # Root of app data
├── <app-id>/
│   ├── config/                  # Compose files, env, secrets (0700 nos:nos)
│   │   ├── docker-compose.yml
│   │   ├── .env
│   │   └── secrets/
│   └── data/                    # App persistent data (Btrfs subvol if available)
└── .snapshots/                  # Snapshot storage
    └── <app-id>/
        └── <timestamp>-<name>/

/var/lib/nos/apps/state/         # Runtime state (0700 nosd:nosd)
/etc/nos/apps/                   # Catalog config (0700 nosd:nosd)
/usr/share/nos/apps/       # Built-in templates (read-only)
```

## Security Model

### Container Defaults

All containers run with:
- `no-new-privileges`: Prevent privilege escalation
- `read_only: true`: Read-only root filesystem (unless required)
- Resource limits: CPU/memory constraints from templates
- Port restrictions: Only explicitly mapped ports
- User namespaces: UID/GID remapping when possible

### Isolation

- Deny host PID/NET/IPC namespaces by default
- Deny privileged mode unless `needs_privileged: true`
- Secrets mounted read-only from `/srv/apps/<id>/config/secrets/`
- AppArmor/SELinux profiles when available

### Docker Configuration

```json
{
  "log-driver": "local",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  },
  "storage-driver": "overlay2",
  "iptables": true,
  "live-restore": true,
  "userland-proxy": false,
  "default-ulimits": {
    "nofile": {
      "Name": "nofile",
      "Hard": 64000,
      "Soft": 64000
    }
  },
  "shutdown-timeout": 25,
  "default-address-pools": [
    {
      "base": "172.28.0.0/16",
      "size": 24
    }
  ]
}
```

## Snapshot Management

### Btrfs Mode (Preferred)

When `/srv/apps/<id>/data` is on Btrfs:
1. Data directory created as subvolume
2. Instant read-only snapshots via `btrfs subvolume snapshot`
3. Atomic rollback by replacing subvolume

### Rsync Fallback

For non-Btrfs filesystems:
1. Full directory copy with `rsync -aHAXS`
2. Slower but compatible with any filesystem
3. Same rollback interface

### Operations

```bash
# Create pre-change snapshot
/usr/lib/nos/apps/nos-app-snapshot.sh snapshot-pre <app-id> <name>

# Rollback to snapshot
/usr/lib/nos/apps/nos-app-snapshot.sh rollback <app-id> <timestamp>

# List snapshots
/usr/lib/nos/apps/nos-app-snapshot.sh list-snapshots <app-id>

# Prune old snapshots (keep N most recent)
/usr/lib/nos/apps/nos-app-snapshot.sh prune-snapshots <app-id> [keep-count]
```

## Systemd Integration

### Templated Service Unit

`nos-app@.service` provides:
- Automatic startup/shutdown with Docker
- Health checks and restart on failure
- Resource isolation via systemd slices
- Proper dependency ordering

### Usage

```bash
# Start an app
systemctl start nos-app@<app-id>.service

# Enable auto-start
systemctl enable nos-app@<app-id>.service

# View logs
journalctl -u nos-app@<app-id>.service

# Stop an app
systemctl stop nos-app@<app-id>.service
```

## CLI Helpers

### Docker Operations

```bash
# Check Docker status
/usr/lib/nos/apps/nos-app-helper.sh docker-ok

# Start app containers
/usr/lib/nos/apps/nos-app-helper.sh compose-up /srv/apps/<id>/config

# Stop app containers
/usr/lib/nos/apps/nos-app-helper.sh compose-down /srv/apps/<id>/config

# List containers
/usr/lib/nos/apps/nos-app-helper.sh compose-ps /srv/apps/<id>/config

# Get container health
/usr/lib/nos/apps/nos-app-helper.sh health-read <container-name>
```

### App Management

```bash
# Pre-start checks
/usr/lib/nos/apps/nos-app-helper.sh pre-start <app-id>

# List installed apps
/usr/lib/nos/apps/nos-app-helper.sh list-apps

# Get app status
/usr/lib/nos/apps/nos-app-helper.sh app-status <app-id>
```

## App Templates

Example template structure:

```json
{
  "id": "nextcloud",
  "name": "Nextcloud",
  "version": "28.0",
  "description": "Self-hosted file sync and collaboration",
  "icon": "nextcloud.svg",
  "category": "productivity",
  "compose": {
    "version": "3.8",
    "services": {
      "app": {
        "image": "nextcloud:28-apache",
        "restart": "unless-stopped",
        "environment": {
          "POSTGRES_HOST": "db",
          "POSTGRES_DB": "nextcloud",
          "POSTGRES_USER": "nextcloud"
        },
        "volumes": [
          "./data:/var/www/html"
        ],
        "security_opt": [
          "no-new-privileges:true"
        ]
      },
      "db": {
        "image": "postgres:15-alpine",
        "restart": "unless-stopped",
        "environment": {
          "POSTGRES_DB": "nextcloud",
          "POSTGRES_USER": "nextcloud"
        },
        "volumes": [
          "./data/db:/var/lib/postgresql/data"
        ],
        "security_opt": [
          "no-new-privileges:true"
        ]
      }
    }
  },
  "requirements": {
    "min_memory": 512,
    "min_disk": 10240,
    "needs_privileged": false,
    "ports": [
      {"container": 80, "host": 8080, "protocol": "tcp"}
    ]
  }
}
```

## Testing

Run the smoke test to verify installation:

```bash
sudo bash scripts/smoke-apps-runtime.sh
```

Tests include:
- Docker installation and configuration
- Directory structure and permissions
- CLI helper functionality
- App deployment lifecycle
- Snapshot/rollback operations
- Service management

## Future Enhancements

- [ ] GPU passthrough support
- [ ] Multi-node Docker Swarm mode
- [ ] Kubernetes runtime option
- [ ] App marketplace with ratings
- [ ] Automated backup to S3/B2
- [ ] Resource usage monitoring
- [ ] App-to-app networking policies
- [ ] Certificate management for apps
