#!/bin/bash
# Build script for macOS
# Requires: Go, Node.js, Wails CLI, Xcode Command Line Tools

set -e

echo "Building NithronSync for macOS..."

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

# Install frontend dependencies
echo "Installing frontend dependencies..."
cd frontend
npm install
cd ..

# Build universal binary (Intel + Apple Silicon)
echo "Building application..."
wails build -platform darwin/universal -o NithronSync.app

if [ $? -eq 0 ]; then
    echo "Build successful!"
    echo "Output: build/bin/NithronSync.app"
else
    echo "Build failed!"
    exit 1
fi

# Code sign (requires Apple Developer certificate)
if [ -n "$APPLE_DEVELOPER_ID" ]; then
    echo "Code signing..."
    codesign --force --deep --sign "$APPLE_DEVELOPER_ID" build/bin/NithronSync.app
    
    # Notarize (requires App Store Connect API key)
    if [ -n "$APPLE_API_KEY" ]; then
        echo "Notarizing..."
        xcrun notarytool submit build/bin/NithronSync.app \
            --apple-id "$APPLE_ID" \
            --password "$APPLE_APP_PASSWORD" \
            --team-id "$APPLE_TEAM_ID" \
            --wait
        
        xcrun stapler staple build/bin/NithronSync.app
    fi
fi

# Create DMG
echo "Creating DMG..."
mkdir -p dist

if command -v create-dmg &> /dev/null; then
    create-dmg \
        --volname "NithronSync" \
        --volicon "build/appicon.icns" \
        --window-pos 200 120 \
        --window-size 600 400 \
        --icon-size 100 \
        --icon "NithronSync.app" 150 190 \
        --hide-extension "NithronSync.app" \
        --app-drop-link 450 185 \
        "dist/NithronSync-1.0.0.dmg" \
        "build/bin/NithronSync.app"
else
    # Fallback to hdiutil
    hdiutil create -volname "NithronSync" \
        -srcfolder "build/bin/NithronSync.app" \
        -ov -format UDZO \
        "dist/NithronSync-1.0.0.dmg"
fi

echo "DMG created: dist/NithronSync-1.0.0.dmg"

