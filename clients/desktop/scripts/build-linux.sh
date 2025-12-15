#!/bin/bash
# Build script for Linux
# Requires: Go, Node.js, Wails CLI, and GTK/WebKit dev libraries

set -e

echo "Building NithronSync for Linux..."

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    exit 1
fi

if ! command -v node &> /dev/null; then
    echo "Error: Node.js is not installed"
    exit 1
fi

if ! command -v wails &> /dev/null; then
    echo "Installing Wails CLI..."
    go install github.com/wailsapp/wails/v2/cmd/wails@latest
fi

# Check for GTK dependencies
if ! pkg-config --exists gtk+-3.0 webkit2gtk-4.0; then
    echo "Warning: GTK/WebKit development libraries may be missing"
    echo "Install with: sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"
fi

# Install frontend dependencies
echo "Installing frontend dependencies..."
cd frontend
npm install
cd ..

# Build
echo "Building application..."
wails build -platform linux/amd64 -o NithronSync

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo "Output: build/bin/NithronSync"
else
    echo "Build failed!"
    exit 1
fi

# Create AppImage (optional)
if command -v appimagetool &> /dev/null; then
    echo "Creating AppImage..."
    mkdir -p AppDir/usr/bin
    cp build/bin/NithronSync AppDir/usr/bin/
    
    cat > AppDir/NithronSync.desktop << EOF
[Desktop Entry]
Name=NithronSync
Exec=NithronSync
Icon=nithron-sync
Type=Application
Categories=Utility;FileTools;Network;
EOF
    
    cp build/appicon.png AppDir/nithron-sync.png
    
    appimagetool AppDir NithronSync-x86_64.AppImage
    rm -rf AppDir
    
    echo "AppImage created: NithronSync-x86_64.AppImage"
fi

# Create .deb package (optional)
create_deb() {
    echo "Creating .deb package..."
    
    PKG_DIR="nithron-sync_1.0.0_amd64"
    mkdir -p "$PKG_DIR/DEBIAN"
    mkdir -p "$PKG_DIR/usr/bin"
    mkdir -p "$PKG_DIR/usr/share/applications"
    mkdir -p "$PKG_DIR/usr/share/icons/hicolor/256x256/apps"
    
    # Control file
    cat > "$PKG_DIR/DEBIAN/control" << EOF
Package: nithron-sync
Version: 1.0.0
Section: utils
Priority: optional
Architecture: amd64
Depends: libgtk-3-0, libwebkit2gtk-4.0-37
Maintainer: Nithron <hello@nithron.com>
Description: NithronSync - File Synchronization for NithronOS
 Cross-platform file sync client for NithronOS servers.
EOF
    
    # Copy binary
    cp build/bin/NithronSync "$PKG_DIR/usr/bin/nithron-sync"
    chmod +x "$PKG_DIR/usr/bin/nithron-sync"
    
    # Desktop entry
    cat > "$PKG_DIR/usr/share/applications/nithron-sync.desktop" << EOF
[Desktop Entry]
Name=NithronSync
Comment=File Synchronization for NithronOS
Exec=/usr/bin/nithron-sync
Icon=nithron-sync
Terminal=false
Type=Application
Categories=Utility;FileTools;Network;
EOF
    
    # Icon
    cp build/appicon.png "$PKG_DIR/usr/share/icons/hicolor/256x256/apps/nithron-sync.png"
    
    # Build package
    dpkg-deb --build "$PKG_DIR"
    rm -rf "$PKG_DIR"
    
    echo ".deb package created: ${PKG_DIR}.deb"
}

if command -v dpkg-deb &> /dev/null; then
    create_deb
fi

