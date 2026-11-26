# App Catalog

The nOS App Catalog provides one-click installation and management of containerized applications through a curated marketplace and self-service portal.

## Overview

The App Catalog is built on Docker Engine and Docker Compose v2, providing:

- **Curated Marketplace**: Pre-configured applications ready to deploy
- **One-Click Installation**: Simple wizard-driven deployment
- **Lifecycle Management**: Start, stop, upgrade, rollback operations
- **Health Monitoring**: Automatic health checks and status reporting
- **Snapshot Integration**: Btrfs-aware snapshots before changes
- **Reverse Proxy**: Automatic Caddy integration for web access
- **Security by Default**: Sandboxed containers with resource limits

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Web UI (React)                       │
├─────────────────────────────────────────────────────────┤
│                   nosd API Server                       │
├─────────────┬───────────────────────────────────────────┤
│  App Manager │          nos-agent (privileged)          │
├──────────────┴───────────────────────────────────────────┤
│                  Docker Engine + Compose                │
├─────────────────────────────────────────────────────────┤
│              Btrfs/ext4 Storage + Snapshots            │
└─────────────────────────────────────────────────────────┘
```

## Installation

The App Catalog is installed automatically with nOS. The required packages are:

1. **nos-apps-runtime**: Docker runtime and utilities
2. **nos-apps**: Catalog data and templates
3. **nosd**: Backend API server
4. **nos-web**: Frontend UI

```bash
# Install all required packages (Debian/Ubuntu)
apt-get install nos-apps-runtime nos-apps nosd nos-web

# Verify Docker is running
docker info

# Access the web UI
https://your-server/apps
```

## Using the App Catalog

### Installing an App

1. Navigate to **Apps** in the web interface
2. Browse or search for your desired application
3. Click **Install** on the app card
4. Complete the installation wizard:
   - Review app permissions and requirements
   - Configure parameters (ports, passwords, storage)
   - Review the generated configuration
   - Click **Install** to deploy

### Managing Installed Apps

From the **Installed Apps** tab, you can:

- **Start/Stop**: Control app runtime state
- **Restart**: Restart all app containers
- **View Logs**: Stream live container logs
- **Open App**: Access the app's web interface
- **Upgrade**: Update to newer versions
- **Rollback**: Restore from snapshots
- **Uninstall**: Remove the app and optionally its data

### App Health Monitoring

Apps are continuously monitored for health:

- **Container Status**: Docker container state
- **HTTP Checks**: Optional endpoint monitoring
- **Resource Usage**: CPU and memory consumption
- **Log Analysis**: Error detection in logs

Health states:
- 🟢 **Healthy**: All checks passing
- 🟡 **Starting**: Application is initializing
- 🔴 **Unhealthy**: One or more checks failing
- ⚫ **Stopped**: Application is not running

## Available Applications

### Productivity & Collaboration
- **Nextcloud**: File sync, sharing, and collaboration
- **Code Server**: VS Code in the browser
- **n8n**: Workflow automation platform

### Media & Entertainment
- **Jellyfin**: Media server for movies, TV, and music
- **Plex**: Media server with transcoding (coming soon)
- **PhotoPrism**: AI-powered photo management (coming soon)

### Development & Testing
- **Whoami**: Simple test application
- **GitLab**: DevOps platform (coming soon)
- **Jenkins**: CI/CD automation (coming soon)

### Home Automation
- **Home Assistant**: Smart home hub (coming soon)
- **Node-RED**: Flow-based programming (coming soon)

## App Configuration

### Parameters

Each app defines parameters through a JSON Schema that generates the configuration form:

```json
{
  "type": "object",
  "properties": {
    "PORT": {
      "type": "string",
      "title": "Web Port",
      "default": "8080",
      "pattern": "^[0-9]{1,5}$"
    },
    "PASSWORD": {
      "type": "string",
      "title": "Admin Password",
      "format": "password",
      "minLength": 8
    }
  },
  "required": ["PASSWORD"]
}
```

### Storage

Apps store data in organized directories:

```
/srv/apps/
├── <app-id>/
│   ├── config/          # Rendered compose files (protected)
│   │   ├── compose.yaml
│   │   ├── .env
│   │   └── secrets/
│   └── data/           # Persistent app data
│       ├── config/     # App configuration
│       ├── cache/      # Temporary data
│       └── media/      # User content
```

### Networking

Apps can expose services through:

1. **Direct Ports**: Map container ports to host ports
2. **Reverse Proxy**: Access via `https://server/apps/<app-id>`
3. **Custom Domains**: Configure virtual hosts in Caddy

## Snapshots and Rollback

Before any destructive operation (upgrade, config change, deletion), the system automatically creates snapshots:

### Automatic Snapshots

- **Pre-upgrade**: Before applying updates
- **Pre-config**: Before configuration changes
- **Pre-delete**: Before removing an app

### Manual Snapshots

Create snapshots through the UI or API:

```bash
# Via API
curl -X POST https://server/api/v1/apps/<app-id>/snapshot \
  -H "Authorization: Bearer $TOKEN"
```

### Rollback

Restore from any snapshot:

1. Go to app details page
2. Click **Snapshots** tab
3. Select a snapshot
4. Click **Rollback**
5. Confirm the operation

## Security

### Container Isolation

All apps run with security constraints:

- **no-new-privileges**: Prevent privilege escalation
- **Read-only root**: Immutable container filesystem
- **User namespaces**: Isolated user IDs
- **Network isolation**: Private Docker networks
- **Resource limits**: CPU and memory constraints

### Secrets Management

Sensitive data is handled securely:

- Secrets stored in `/srv/apps/<app-id>/config/secrets/`
- File permissions set to 0400 (read-only for owner)
- Encrypted at rest on Btrfs with encryption
- Never logged or exposed in UI

### Access Control

App management requires authentication:

- **apps:manage** role: Install, upgrade, delete apps
- **apps:view** role: View status and logs only
- **Audit logging**: All operations logged with user ID

## Authoring App Templates

### Template Structure

Create custom app templates with:

```
templates/<app-id>/
├── compose.yaml     # Docker Compose file
├── schema.json      # Parameter schema
├── README.md        # User documentation
├── icon.svg         # App icon (optional)
├── post_install.sh  # Post-install script (optional)
└── upgrade.sh       # Upgrade script (optional)
```

### Compose Template

Use environment variables for configuration:

```yaml
version: '3.9'

services:
  app:
    image: myapp:${VERSION:-latest}
    ports:
      - "${PORT:-8080}:8080"
    volumes:
      - ./data:/data
    environment:
      - DB_HOST=${DB_HOST:-localhost}
      - DB_PASSWORD=${DB_PASSWORD}
    labels:
      - "nos.app.id=myapp"
      - "nos.app.name=My Application"
```

### Schema Definition

Define parameters with JSON Schema:

```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "title": "My App Configuration",
  "properties": {
    "VERSION": {
      "type": "string",
      "title": "Version",
      "enum": ["latest", "1.0", "2.0"],
      "default": "latest"
    },
    "PORT": {
      "type": "string",
      "title": "Port",
      "pattern": "^[0-9]{1,5}$",
      "default": "8080"
    },
    "DB_PASSWORD": {
      "type": "string",
      "title": "Database Password",
      "format": "password",
      "minLength": 8
    }
  },
  "required": ["DB_PASSWORD"]
}
```

### Catalog Entry

Add to `catalog.yaml`:

```yaml
entries:
  - id: myapp
    name: My Application
    version: "1.0"
    description: Description of my application
    categories:
      - productivity
    icon: icons/myapp.svg
    compose: templates/myapp/compose.yaml
    schema: templates/myapp/schema.json
    health:
      type: http
      url: "http://myapp:8080/health"
      interval_s: 30
```

## Troubleshooting

### App Won't Start

1. Check container logs:
   ```bash
   docker logs nos-app-<app-id>-<service>-1
   ```

2. Verify Docker is running:
   ```bash
   systemctl status docker
   docker info
   ```

3. Check port conflicts:
   ```bash
   netstat -tlnp | grep <port>
   ```

### Health Checks Failing

1. Test health endpoint manually:
   ```bash
   curl http://localhost:<port>/health
   ```

2. Review health check configuration in compose.yaml

3. Check container resource usage:
   ```bash
   docker stats nos-app-<app-id>-<service>-1
   ```

### Cannot Access App

1. Verify Caddy configuration:
   ```bash
   cat /etc/caddy/Caddyfile.d/app-<app-id>.caddy
   caddy validate
   ```

2. Test reverse proxy:
   ```bash
   curl -I https://server/apps/<app-id>/
   ```

3. Check firewall rules:
   ```bash
   iptables -L -n | grep <port>
   ```

## API Reference

### Catalog Operations

- `GET /api/v1/apps/catalog` - List available apps
- `POST /api/v1/apps/catalog/sync` - Sync remote catalogs

### App Management

- `GET /api/v1/apps/installed` - List installed apps
- `GET /api/v1/apps/:id` - Get app details
- `POST /api/v1/apps/install` - Install new app
- `POST /api/v1/apps/:id/upgrade` - Upgrade app
- `POST /api/v1/apps/:id/start` - Start app
- `POST /api/v1/apps/:id/stop` - Stop app
- `POST /api/v1/apps/:id/restart` - Restart app
- `POST /api/v1/apps/:id/rollback` - Rollback to snapshot
- `DELETE /api/v1/apps/:id` - Uninstall app

### Monitoring

- `GET /api/v1/apps/:id/logs` - Stream container logs
- `GET /api/v1/apps/:id/events` - Get app events
- `POST /api/v1/apps/:id/health` - Force health check

## Best Practices

1. **Always use strong passwords** for app admin interfaces
2. **Create snapshots** before major changes
3. **Monitor resource usage** to prevent overload
4. **Use HTTPS** when exposing apps to the internet
5. **Keep apps updated** for security patches
6. **Test in development** before production deployment
7. **Document custom apps** with clear README files
8. **Use environment variables** for configuration
9. **Set resource limits** appropriate for your hardware
10. **Regular backups** of app data directories

## See Also

- [App Runtime Architecture](runtime.md)
- [API Documentation](../api/openapi.json)
- [Security Model](../security-model.md)
- [Storage Pools](../storage/pools.md)
