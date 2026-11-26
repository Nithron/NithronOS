# Changelog

All notable changes to nOS (formerly NithronOS) will be documented in this file.

## [Unreleased]

### Changed
- **NithronOS is now nOS** (NithronOS stays the long form). The repository moved to github.com/Nithron/nOS, and identifiers changed with it: the meta package is `nos` (a transitional `nithronos` package pulls it in), files moved from `/usr/share/nithronos`, `/usr/lib/nithronos`, `/var/lib/nithronos` and `/etc/nithronos` to the `nos` paths, TLS certificates to `/etc/ssl/nos`, units are named `nos-*`, the default hostname is `nos` (`nos.local`), os-release says `ID=nos`, Prometheus metrics are prefixed `nos_` and the sync clients use `nos://` links. Upgrading moves an existing system to the new names automatically (`/usr/lib/nos/migrate-from-nithronos.sh`, run by the nosd package); its hostname is kept.

## [v0.9.5-pre-alpha] - 2024-12-21

### 🎉 Major Features
- **Real-time Dashboard**: Complete dashboard rewrite with live data updates at 1Hz
  - System health metrics (CPU, memory, load, uptime) with smooth animations
  - Storage usage visualization with pie charts
  - Disk health monitoring with SMART status
  - Recent activity feed with event streaming
  - Network shares and installed apps status
  - Maintenance operations tracking
- **Health Monitoring System**: New dedicated health pages with real-time metrics
  - System Health page with CPU, memory, network, and disk I/O monitoring
  - Disk Health page with SMART data and temperature monitoring
  - Monitoring Dashboard with live charts and service status
- **Debian Installer Integration**: Full working Debian Installer flow
  - BIOS and UEFI boot support
  - Automated preseed configuration
  - Non-blocking banner display
  - Service auto-enablement post-install

### 🚀 Improvements
- **Setup & First-time Experience**:
  - Fixed welcome screen layout and centering issues
  - Improved step navigation with visual indicators
  - Telemetry opt-in/opt-out implementation
  - Better loading states and error handling
- **Storage System**:
  - Real-time storage metrics with pool status
  - Data scrub and balance operation tracking
  - Pool health visualization
  - Device management improvements
- **Shares Management**:
  - Fixed "Share Not Found" error on creation
  - Proper navigation after share creation/editing
  - SMB/NFS/AFP protocol support
- **Snapshots & Schedules**:
  - Improved UI for snapshot and schedule creation buttons
  - Better visual consistency across the interface
- **Apps Catalog**:
  - Modern card-based UI with category filtering
  - Working "Sync Catalogs" functionality
  - Grid/list view toggle
  - Installation status tracking
- **Remote Backup**:
  - Destination management with S3/SFTP support
  - Backup job scheduling and monitoring
  - Real-time backup statistics
- **Settings Page**:
  - Removed duplicate menu items
  - Added Appearance, Notifications, Privacy & Security sections
  - Regional settings and customization options

### 🐛 Bug Fixes
- Fixed disk tab crash ("j?.find is not a function")
- Resolved array handling issues preventing crashes
- Fixed TypeScript errors across multiple components
- Corrected framer-motion animation variants typing
- Fixed Badge component to support destructive variant
- Fixed Tabs component to support defaultValue prop
- Resolved all golangci-lint errcheck issues

### 🔧 Technical Changes
- **Backend**:
  - Added dashboard aggregator endpoint (`/api/dashboard`)
  - Implemented real-time health endpoints using gopsutil
  - Added proper error handling for JSON encoding
  - Created modular API handlers for dashboard widgets
- **Frontend**:
  - Migrated to React Query for all data fetching
  - Implemented 1Hz refresh with smooth updates
  - Added proper TypeScript interfaces for API responses
  - Created reusable hooks for dashboard and health data
  - Added missing UI components (ScrollArea, Table)
- **Build System**:
  - Fixed Debian Installer kernel/initrd fetching
  - Added CI smoke tests for installer validation
  - Improved ISO build process with proper error handling

### 📚 Documentation
- Updated mkdocs configuration with Discord and Patreon links
- Added comprehensive README with all documentation links
- Created Debian Installer documentation
- Added recovery and troubleshooting guides

### 🔄 Dependencies
- Added `github.com/shirou/gopsutil/v3` for system metrics
- Updated React Query configuration for optimal performance
- Added Radix UI scroll-area component

### Known Issues
- Some advanced Btrfs features (send/receive) still in development
- ZFS support pending platform licensing resolution
- Remote access features planned for future releases

---

## [v0.1.0-pre-alpha] - 2024-12-01

### Initial Release
- Disk discovery & SMART monitoring
- Btrfs: create/import, snapshots, basic usage reporting
- Shares: SMB/NFS with simple ACLs + UI wizard, SMB user management
- App catalog: Docker/Compose one-click installs
- Firewall: LAN-only default, Remote Access wizard with 2FA + rollback/backup
- Updates: snapshot-before-update, rollback UI/API, retention (keep 5)
- ISO build workflow + QEMU smoke test
- Tests across backend/agent/web

### Known Limitations
- Btrfs send/receive not yet implemented
- Root filesystem rollback pending
- Per-app pre/post hooks in development
- Advanced retention policies planned