#!/bin/bash
# NithronSync uninstaller for Linux

echo "Uninstalling NithronSync..."

# Stop service if running
systemctl --user stop nithron-sync 2>/dev/null || true
systemctl --user disable nithron-sync 2>/dev/null || true

# Remove binary
sudo rm -f /usr/bin/nithron-sync

# Remove desktop entries
rm -f "$HOME/.local/share/applications/nithron-sync.desktop"
rm -f "$HOME/.config/autostart/nithron-sync.desktop"

# Remove systemd service
rm -f "$HOME/.config/systemd/user/nithron-sync.service"
systemctl --user daemon-reload 2>/dev/null || true

echo "NithronSync uninstalled."
echo ""
echo "Note: Configuration and sync data were NOT removed."
echo "To remove them manually:"
echo "  rm -rf ~/.config/NithronSync"
echo "  rm -rf ~/.local/share/NithronSync"
echo "  rm -rf ~/NithronSync  # Your synced files"

