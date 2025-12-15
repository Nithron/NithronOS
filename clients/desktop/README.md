# NithronSync Desktop Client

Cross-platform desktop client for NithronSync file synchronization.

## Features

- **Cross-Platform**: Windows, Linux, and macOS support
- **System Tray**: Runs in the background with system tray integration
- **Real-time Sync**: Watches for file changes and syncs automatically
- **Delta Sync**: Only transfers changed portions of files
- **Conflict Resolution**: Handles conflicts with configurable policies
- **Bandwidth Control**: Limit sync speed to preserve network capacity

## Building

### Prerequisites

- Go 1.23+
- Node.js 20+
- Wails CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`)

### Platform-specific requirements

**Windows:**
- Visual Studio Build Tools or MinGW

**Linux:**
- GTK 3 development libraries
- WebKit2GTK development libraries
```bash
# Ubuntu/Debian
sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev

# Fedora
sudo dnf install gtk3-devel webkit2gtk4.0-devel
```

**macOS:**
- Xcode Command Line Tools
```bash
xcode-select --install
```

### Build Commands

```bash
# Install dependencies
make install-deps

# Development mode (hot reload)
make dev

# Build for current platform
make build

# Build for specific platform
make build-windows
make build-linux
make build-darwin

# Build all platforms
make build-all
```

## Installation

### Windows

1. Download `NithronSync-Setup.exe` from releases
2. Run the installer
3. Follow the setup wizard

Or use the portable version:
1. Download `NithronSync.exe`
2. Run directly (no installation required)

### Linux

**AppImage:**
```bash
chmod +x NithronSync-x86_64.AppImage
./NithronSync-x86_64.AppImage
```

**Debian/Ubuntu:**
```bash
sudo dpkg -i nithron-sync_1.0.0_amd64.deb
```

**From source:**
```bash
./scripts/build-linux.sh
sudo cp build/bin/NithronSync /usr/bin/nithron-sync
```

### macOS

1. Download `NithronSync-1.0.0.dmg`
2. Open the DMG
3. Drag NithronSync to Applications
4. Open from Applications (you may need to right-click → Open the first time)

## Configuration

Configuration is stored in:
- **Windows**: `%APPDATA%\NithronSync\config.json`
- **Linux**: `~/.config/NithronSync/config.json`
- **macOS**: `~/Library/Application Support/NithronSync/config.json`

### Initial Setup

1. Open NithronSync
2. Enter your NithronOS server URL
3. Paste the device token from NithronOS (Settings → Sync → Devices → Add)
4. Choose your sync folder location
5. Click Connect

## Usage

### System Tray

NithronSync runs in the system tray by default. The icon indicates sync status:
- 🟢 Green cloud: Synced and up to date
- 🔵 Blue cloud: Syncing in progress
- 🟡 Yellow cloud: Paused or offline
- 🔴 Red cloud: Error

Right-click the tray icon for options:
- Open NithronSync
- Open Sync Folder
- Pause/Resume Sync
- Sync Now
- Settings
- Quit

### Command Line

```bash
# Start with window visible
nithron-sync

# Start minimized to tray
nithron-sync --minimized

# Run as background daemon
nithron-sync --daemon

# Show version
nithron-sync --version

# Show help
nithron-sync --help
```

## Troubleshooting

### Logs

Logs are stored in:
- **Windows**: `%LOCALAPPDATA%\NithronSync\logs\`
- **Linux**: `~/.local/share/nithron-sync/logs/`
- **macOS**: `~/Library/Logs/NithronSync/`

### Common Issues

**Can't connect to server:**
- Verify server URL is correct
- Check if server is reachable (try in browser)
- Ensure HTTPS certificate is valid

**Files not syncing:**
- Check if sync is paused
- Verify file isn't excluded by pattern
- Check available disk space

**High CPU usage:**
- Large number of files being watched
- Consider using selective sync

## Development

### Project Structure

```
clients/desktop/
├── app/                 # Go application logic
│   ├── app.go          # Main app struct and methods
│   └── tray.go         # System tray management
├── frontend/           # React frontend
│   ├── src/
│   │   ├── App.tsx     # Main React component
│   │   └── ...
│   └── package.json
├── build/              # Build assets
├── installer/          # Platform installers
├── scripts/            # Build scripts
├── main.go             # Entry point
├── wails.json          # Wails configuration
└── Makefile            # Build commands
```

### Adding Features

1. Backend (Go): Add methods to `app/app.go`
2. Frontend (React): Update `frontend/src/App.tsx`
3. Build: `make dev` for hot-reload development

## License

NithronOS Community License. See LICENSE file.

