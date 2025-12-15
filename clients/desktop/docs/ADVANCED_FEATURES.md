# Advanced Features Roadmap

This document describes advanced features that are planned but require additional platform-specific implementation.

## Shell Extensions

### Windows Explorer Overlay Icons

Status: **Planned for v1.1**

Shell extensions require a native Windows COM component to display sync status icons on files/folders.

Implementation approach:
1. Create a C++ shell extension DLL implementing `IShellIconOverlayIdentifier`
2. Register the DLL as a shell extension
3. Use named pipes or shared memory to communicate with NithronSync

Reference: [Microsoft Shell Icon Overlay Handlers](https://docs.microsoft.com/en-us/windows/win32/shell/how-to-implement-icon-overlay-handlers)

### macOS Finder Extension

Status: **Planned for v1.1**

Finder extensions require a native macOS app extension.

Implementation approach:
1. Create a Finder Sync Extension (Swift/Objective-C)
2. Use XPC services to communicate with NithronSync
3. Implement `FIFinderSyncProtocol` for badges and context menus

Reference: [Apple Finder Sync Extension](https://developer.apple.com/documentation/findersync)

### Linux Nautilus/Dolphin Extensions

Status: **Planned for v1.1**

**Nautilus (GNOME Files):**
- Python extension using `gi.repository.Nautilus`
- Communicates via D-Bus or Unix socket

**Dolphin (KDE):**
- C++ plugin using KIO framework
- Alternative: overlay via `.directory` files

## Auto-Updates

Status: **Planned for v1.1**

Implementation approach:

### Windows
- Use Squirrel.Windows or WinSparkle
- Background download + update on next launch
- Code signing for security

### macOS
- Use Sparkle framework
- Notarization required
- DMG or pkg auto-update

### Linux
- AppImageUpdate for AppImages
- Repository integration for .deb
- Flatpak for sandboxed updates

## Context Menu Integration

Status: **Planned for v1.1**

Right-click menu options:
- "Share Link" - Generate sharing link
- "View on NithronOS" - Open in web UI
- "View History" - Show file versions
- "Resolve Conflict" - Conflict resolution dialog

## Selective Sync

Status: **Implemented (partial)**

Current: All enabled shares sync fully
Planned: 
- Folder-level selection
- Smart sync (download on access)
- Virtual files (Windows Cloud Files API / macOS FUSE)

## Bandwidth Scheduling

Status: **Planned for v1.2**

Allow scheduling sync during specific hours:
- Peak/off-peak bandwidth limits
- Pause during work hours
- Priority by file type

## Multi-Account Support

Status: **Planned for v1.2**

Support connecting to multiple NithronOS servers:
- Switch between accounts
- Separate sync folders per account
- Unified activity view

## Contributing

These features require platform expertise. If you'd like to contribute:

1. Check the issue tracker for existing work
2. Discuss implementation approach in the issue
3. Submit a PR with tests

Platform-specific code should be in separate directories:
- `shell-ext/windows/` - Windows COM components
- `shell-ext/macos/` - Finder Sync extension
- `shell-ext/linux/` - Nautilus/Dolphin plugins

