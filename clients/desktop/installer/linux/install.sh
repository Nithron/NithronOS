#!/bin/bash
# NithronSync installer for Linux

set -e

INSTALL_DIR="/usr/bin"
CONFIG_DIR="$HOME/.config/NithronSync"
DATA_DIR="$HOME/.local/share/NithronSync"
SYNC_DIR="$HOME/NithronSync"

echo "Installing NithronSync..."

# Create directories
mkdir -p "$CONFIG_DIR"
mkdir -p "$DATA_DIR"
mkdir -p "$SYNC_DIR"

# Copy binary (requires sudo for /usr/bin)
if [ -f "NithronSync" ]; then
    sudo cp NithronSync "$INSTALL_DIR/nithron-sync"
    sudo chmod +x "$INSTALL_DIR/nithron-sync"
fi

# Install desktop file
mkdir -p "$HOME/.local/share/applications"
cat > "$HOME/.local/share/applications/nithron-sync.desktop" << EOF
[Desktop Entry]
Name=NithronSync
Comment=File Synchronization for NithronOS
Exec=/usr/bin/nithron-sync
Icon=nithron-sync
Terminal=false
Type=Application
Categories=Utility;FileTools;Network;
StartupNotify=true
EOF

# Install autostart entry
mkdir -p "$HOME/.config/autostart"
cat > "$HOME/.config/autostart/nithron-sync.desktop" << EOF
[Desktop Entry]
Name=NithronSync
Comment=Start NithronSync on login
Exec=/usr/bin/nithron-sync --minimized
Icon=nithron-sync
Terminal=false
Type=Application
X-GNOME-Autostart-enabled=true
EOF

# Install systemd user service (optional)
mkdir -p "$HOME/.config/systemd/user"
cat > "$HOME/.config/systemd/user/nithron-sync.service" << EOF
[Unit]
Description=NithronSync File Synchronization
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/nithron-sync --daemon
Restart=on-failure
RestartSec=10

[Install]
WantedBy=default.target
EOF

echo "NithronSync installed successfully!"
echo ""
echo "To start NithronSync:"
echo "  - Run 'nithron-sync' from the terminal"
echo "  - Or find it in your applications menu"
echo ""
echo "To enable autostart with systemd:"
echo "  systemctl --user enable nithron-sync"
echo "  systemctl --user start nithron-sync"

