# NithronSync Client Setup

This guide covers installing and configuring NithronSync clients on various platforms.

## Prerequisites

Before setting up a client:

1. Ensure your NithronOS server is accessible from the client device
2. Have at least one sync-enabled share configured
3. Have your NithronOS credentials ready (or a registered device token)

## Windows

### Installation

1. Download the NithronSync installer from the web UI (**Settings → Sync → Download Client**)
2. Run `NithronSync-Setup.exe`
3. Follow the installation wizard
4. The app will start automatically after installation

### Initial Setup

1. **Server URL** — Enter your NithronOS server address (e.g., `https://nas.local`)
2. **Authentication** — Choose one:
   - **Web Login** — Opens browser for secure authentication
   - **Device Token** — Paste a token from the web UI
3. **Sync Folder** — Choose where to create your NithronSync folder (default: `%USERPROFILE%\NithronSync`)
4. **Selective Sync** — Choose which shares and folders to sync

### System Tray

NithronSync runs in the system tray:
- **Green cloud** — Synced and up to date
- **Blue arrows** — Syncing in progress
- **Yellow warning** — Some files couldn't sync
- **Red X** — Disconnected or error

Right-click the tray icon for options:
- Open NithronSync folder
- Pause/Resume sync
- View recent activity
- Settings
- Quit

### Sync Folder Structure

```
%USERPROFILE%\NithronSync\
├── Documents/        # Share: Documents
│   ├── Work/
│   └── Personal/
├── Photos/           # Share: Photos
└── .nithronSync/     # Local metadata (don't modify)
```

## Linux

### Installation (AppImage)

```bash
# Download
wget https://your-server/downloads/NithronSync-linux-x86_64.AppImage

# Make executable
chmod +x NithronSync-linux-x86_64.AppImage

# Run
./NithronSync-linux-x86_64.AppImage
```

### Installation (Debian/Ubuntu)

```bash
# Download
wget https://your-server/downloads/nithron-sync_1.0.0_amd64.deb

# Install
sudo dpkg -i nithron-sync_1.0.0_amd64.deb
sudo apt-get install -f  # Install dependencies

# Start
nithron-sync
```

### Installation (Fedora/RHEL)

```bash
# Download
wget https://your-server/downloads/nithron-sync-1.0.0.x86_64.rpm

# Install
sudo dnf install nithron-sync-1.0.0.x86_64.rpm

# Start
nithron-sync
```

### CLI Configuration

```bash
# Configure server
nithron-sync config --server https://nas.local

# Authenticate
nithron-sync auth login
# Opens browser for authentication

# Or use device token
nithron-sync auth token "nos_dt_..."

# Set sync folder
nithron-sync config --sync-folder ~/NithronSync

# Start sync daemon
nithron-sync start
```

### Systemd Service

```bash
# Enable auto-start
systemctl --user enable nithron-sync
systemctl --user start nithron-sync

# Check status
systemctl --user status nithron-sync

# View logs
journalctl --user -u nithron-sync -f
```

## macOS

### Installation

1. Download `NithronSync.dmg` from the web UI
2. Open the DMG and drag NithronSync to Applications
3. Launch NithronSync from Applications
4. Grant required permissions:
   - **Files and Folders** — Access to sync folder
   - **Network** — Connect to NithronOS server

### Menu Bar

NithronSync appears in the menu bar with sync status indicators. Click for quick access to:
- Open sync folder
- Recent activity
- Pause/resume sync
- Preferences

### Configuration

Access preferences via menu bar icon → **Preferences**:

- **Account** — Server URL, sign out
- **Sync** — Folder location, bandwidth limits
- **Selective Sync** — Choose shares/folders
- **Advanced** — Proxy settings, debug logging

## iOS

### Installation

1. Download **NithronSync** from the App Store
2. Open the app and tap **Add Server**
3. Enter your server URL or scan the QR code from web UI
4. Authenticate with your NithronOS credentials or device token
5. Choose which shares to sync

### Features

- **Files App Integration** — Access synced files in the iOS Files app
- **Offline Access** — Mark files/folders for offline availability
- **Photo Backup** — Auto-upload camera roll to a sync share
- **Background Sync** — Syncs in background when connected to Wi-Fi

### Settings

In-app settings:
- **Auto-Sync Photos** — Enable camera roll backup
- **Only on Wi-Fi** — Restrict sync to Wi-Fi networks
- **Offline Files** — Manage offline-available content
- **Storage Usage** — View and clear cached files

## Android

### Installation

1. Download **NithronSync** from Google Play
2. Open the app and tap **Connect to Server**
3. Enter your server URL or scan the QR code
4. Sign in with your NithronOS credentials
5. Grant storage permissions when prompted

### Features

- **SAF Integration** — Access files via Android's Storage Access Framework
- **Auto-Upload** — Automatically sync camera photos
- **Selective Sync** — Choose folders per device
- **Background Service** — Syncs in background

### Settings

- **Sync Frequency** — How often to check for changes
- **Upload on Mobile Data** — Allow uploads on cellular
- **Auto-Upload Folders** — Camera, screenshots, downloads
- **Battery Optimization** — Exempt app for reliable background sync

## WebDAV Clients

NithronSync provides WebDAV access for third-party clients.

### WebDAV URL

```
https://your-server.local/dav/{share_id}/
```

Find the WebDAV URL in **Settings → Sync → WebDAV Access**.

### Authentication

Use your device access token as the password:
- **Username:** Leave empty or use any value
- **Password:** Your device access token (starts with `nos_at_`)

### Compatible Clients

| Platform | Client | Notes |
|----------|--------|-------|
| Windows | Windows Explorer | Map network drive |
| Windows | Cyberduck | File manager |
| macOS | Finder | Connect to Server |
| macOS | Cyberduck | File manager |
| Linux | Nautilus/Files | Connect to Server |
| Linux | Dolphin | Network locations |
| iOS | Files app | Connect to Server |
| Android | Solid Explorer | WebDAV plugin |

### Windows: Map Network Drive

1. Open File Explorer
2. Right-click **This PC** → **Map network drive**
3. Enter WebDAV URL: `https://your-server/dav/share_id/`
4. Check **Connect using different credentials**
5. Username: `token`
6. Password: Your access token

### macOS: Finder

1. Open Finder
2. Press `Cmd+K` (Connect to Server)
3. Enter: `https://your-server/dav/share_id/`
4. Click **Connect**
5. Enter credentials when prompted

## Troubleshooting

### Common Issues

**Can't connect to server**
- Verify server URL is correct
- Check if server is accessible (try in browser)
- Ensure firewall allows HTTPS (port 443)
- Verify SSL certificate is valid

**Sync not starting**
- Check if sync is paused
- Verify internet connection
- Check device token hasn't expired
- Look for errors in client logs

**Files not syncing**
- Check file isn't excluded by pattern
- Verify file size is under limit
- Check available disk space
- Ensure share has sync enabled

**Slow sync**
- Check bandwidth limit settings
- Verify network speed
- Large files use delta sync automatically
- Check server load

### Logs

**Windows:** `%APPDATA%\NithronSync\logs\`
**macOS:** `~/Library/Logs/NithronSync/`
**Linux:** `~/.local/share/nithron-sync/logs/`

### Getting Help

1. Check [Troubleshooting Guide](./troubleshooting.md)
2. Visit [Community Discord](https://discord.gg/qzB37WS5AT)
3. Report issues on [GitHub](https://github.com/Nithronverse/NithronOS/issues)

