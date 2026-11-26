# nOS Updates & Releases

## Overview

nOS provides atomic system updates with automatic rollback capabilities, ensuring your system remains stable and recoverable even if an update fails.

## Key Features

- **Signed APT repositories**: All packages are GPG-signed for security
- **Multiple channels**: Choose between stable and beta update tracks
- **Atomic updates**: Btrfs snapshots ensure updates can be rolled back
- **Automatic rollback**: Failed updates automatically revert to the previous state
- **Progress tracking**: Real-time update progress with detailed logs
- **Snapshot management**: Keep and manage multiple system snapshots

## Update Channels

### Stable Channel (Default)
- Production-ready releases
- Thoroughly tested updates
- Recommended for most users
- Lower update frequency

### Beta Channel
- Early access to new features
- More frequent updates
- May contain bugs
- For testing and development

To change channels:
```bash
# Via API
curl -X POST http://localhost:9000/api/v1/updates/channel \
  -H "Content-Type: application/json" \
  -d '{"channel": "beta"}'

# Via UI
Navigate to Settings → Updates & Releases → Update Channel
```

## Update Process

### 1. Preflight Checks
Before applying updates, the system performs:
- Disk space verification (minimum 2GB required)
- Network connectivity test
- Repository accessibility check
- GPG signature verification

### 2. Snapshot Creation
- Creates Btrfs snapshots of critical subvolumes:
  - `@` (root filesystem)
  - `@etc` (configuration)
  - `@var` (variable data)
- Snapshots are stored in `/.snapshots/nos-update/`
- Each snapshot is timestamped and read-only

### 3. Package Download & Installation
- Downloads packages from the configured channel
- Applies updates using `apt-get dist-upgrade`
- Maintains package dependencies automatically

### 4. Postflight Verification
After installation, the system verifies:
- Critical services are running (nosd, nos-agent, caddy)
- Web UI is accessible
- No system degradation detected

### 5. Automatic Rollback
If postflight checks fail:
- System automatically rolls back to the pre-update snapshot
- Services are restarted
- Update is marked as failed

## Using the Updates UI

### Checking for Updates
1. Navigate to **Settings → Updates & Releases**
2. Click **Check for Updates**
3. Review available updates and their changelog

### Applying Updates
1. Click **Apply Update** when updates are available
2. Monitor real-time progress:
   - Preflight checks
   - Snapshot creation
   - Package download
   - Installation
   - Postflight verification
3. System will notify on completion or failure

### Managing Snapshots
- View all system snapshots in the UI
- Each snapshot shows:
  - Creation timestamp
  - Size on disk
  - Rollback availability
- Delete old snapshots to free space
- Manually rollback to any snapshot

## CLI Usage

### Check for Updates
```bash
curl http://localhost:9000/api/v1/updates/check
```

### Apply Updates
```bash
curl -X POST http://localhost:9000/api/v1/updates/apply
```

### Monitor Progress
```bash
# Get current progress
curl http://localhost:9000/api/v1/updates/progress

# Stream progress (Server-Sent Events)
curl -N http://localhost:9000/api/v1/updates/progress/stream
```

### List Snapshots
```bash
curl http://localhost:9000/api/v1/updates/snapshots
```

### Rollback
```bash
curl -X POST http://localhost:9000/api/v1/updates/rollback \
  -H "Content-Type: application/json" \
  -d '{"snapshot_id": "update-1234567890"}'
```

## Snapshot Retention

By default, the system keeps the 3 most recent update snapshots. Older snapshots are automatically pruned to save disk space.

To modify retention:
- Edit `/etc/nos/update/config.json`
- Set `snapshot_retention` to desired number
- Restart the update service

## Repository Configuration

### Adding the nOS Repository

```bash
# Import GPG key
wget -qO - https://apt.nithronos.com/nos-release.gpg | \
  sudo gpg --dearmor -o /usr/share/keyrings/nos-archive-keyring.gpg

# Add repository
echo "deb [signed-by=/usr/share/keyrings/nos-archive-keyring.gpg] \
  https://apt.nithronos.com stable main" | \
  sudo tee /etc/apt/sources.list.d/nos.list

# Update package lists
sudo apt update
```

### Repository Pinning

APT preferences are automatically configured to prevent cross-channel package drift:
- Packages from the selected channel have priority 1001
- Other nOS packages have priority 900
- System packages maintain default priority

## Safety Features

### Power Loss Protection
- Update state is persisted to `/var/lib/nos-update/state.json`
- On boot, incomplete updates are detected and rolled back
- Ensures system consistency after unexpected shutdowns

### Lock File
- Single-writer lock prevents concurrent updates
- Located at `/var/run/nos-update.lock`
- Automatically released on completion or failure

### Failure Limits
- System tracks consecutive update failures
- After 3 failures, automatic updates are disabled
- Manual intervention required to reset

## Troubleshooting

### Update Fails to Start
- Check disk space: `df -h /`
- Verify network: `ping apt.nithronos.com`
- Check lock file: `ls -la /var/run/nos-update.lock`

### Automatic Rollback Triggered
- Review logs: `/var/log/nos-update.log`
- Check service status:
  ```bash
  systemctl status nosd nos-agent caddy
  ```
- Verify connectivity to Web UI

### Cannot Create Snapshots
- Ensure filesystem is Btrfs: `stat -f -c %T /`
- Check snapshot directory permissions
- Verify sufficient disk space

### Repository Issues
- Verify GPG key: `apt-key list`
- Check sources: `cat /etc/apt/sources.list.d/nos.list`
- Test signature verification:
  ```bash
  apt-get update -o Debug::Acquire::gpgv=true
  ```

## Best Practices

1. **Regular Updates**: Check for updates weekly
2. **Monitor Disk Space**: Keep at least 5GB free for updates
3. **Test Beta Updates**: Use a non-production system for beta channel
4. **Backup Important Data**: While snapshots provide rollback, they're not backups
5. **Review Changelogs**: Understand what's changing before updating

## Security Considerations

- All packages are GPG-signed with nOS release key
- HTTPS is used for all repository communications
- Signature verification is mandatory (cannot be bypassed)
- Failed signature checks prevent any package installation

## API Reference

See [API Documentation](./api/updates.md) for detailed endpoint specifications.

## Telemetry

Telemetry is **disabled by default** and completely optional. If enabled, the following anonymous data is collected:
- System version information
- Update success/failure status
- Update duration
- Rollback frequency

No personal or identifying information is ever collected or transmitted.