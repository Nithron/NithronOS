# Code Server

VS Code in the browser, accessible from anywhere.

## Overview

Code Server allows you to run VS Code on a remote server and access it through your web browser. This gives you a consistent development environment that you can access from any device with a web browser.

## Features

- **Full VS Code Experience**: Get the complete VS Code interface and functionality in your browser
- **Extensions Support**: Install and use VS Code extensions
- **Terminal Access**: Full terminal access with sudo capabilities
- **Persistent Workspaces**: Your projects and settings are saved between sessions
- **Multi-device Access**: Code from any device with a browser
- **Secure Access**: Password protected web interface

## Configuration

### Required Settings

- **Access Password**: A secure password to protect your Code Server instance (minimum 8 characters)

### Optional Settings

- **Web Port**: The port to expose Code Server on (default: 8443)
- **Version**: Specific Code Server version to install (default: latest)
- **Sudo Password**: Password for sudo access in terminal (defaults to access password)
- **Projects Path**: Host path to mount as projects directory
- **Default Workspace**: Default workspace path to open
- **User/Group ID**: UID/GID to run Code Server as (default: 1000)
- **Timezone**: Container timezone (default: UTC)

## Storage

Code Server stores data in the following locations:

- `/home/coder/.config`: VS Code configuration and extensions
- `/home/coder/workspace`: Default workspace directory
- `/home/coder/projects`: Additional projects directory (customizable mount point)

## Security Considerations

- Always use a strong password for access
- Consider using HTTPS when exposing to the internet
- The container runs with no-new-privileges by default
- Sudo access is available but requires password authentication

## Accessing Code Server

After installation, Code Server will be available at:
- Local: `http://localhost:[PORT]`
- Network: `http://[your-server-ip]:[PORT]`
- Via nOS: `https://[your-server]/apps/code-server`

Use the password you configured during installation to log in.

## Tips

- Install extensions by opening the Extensions panel (Ctrl+Shift+X)
- Configure Git credentials for version control
- Use the integrated terminal for command-line operations
- Customize settings through File > Preferences > Settings

## Troubleshooting

If Code Server is not accessible:
1. Check the container is running: `docker ps | grep code-server`
2. Verify the port is not blocked by firewall
3. Check logs: `docker logs nos-app-code-server-app-1`
4. Ensure password meets minimum requirements (8+ characters)
