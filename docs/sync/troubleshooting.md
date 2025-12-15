# NithronSync Troubleshooting

This guide helps resolve common NithronSync issues.

## Connection Issues

### Cannot Connect to Server

**Symptoms:**
- "Unable to connect" error
- Timeout when adding server
- SSL certificate errors

**Solutions:**

1. **Verify server URL**
   ```
   https://your-server.local
   https://192.168.1.100
   ```

2. **Check server is running**
   ```bash
   # On NithronOS server
   systemctl status nosd
   systemctl status caddy-nithronos
   ```

3. **Test connectivity**
   ```bash
   # From client machine
   curl -v https://your-server/api/v1/health
   ```

4. **Check firewall**
   - Ensure port 443 is open
   - Check NithronOS firewall settings in **Settings → Network → Firewall**

5. **SSL certificate issues**
   - For self-signed certs, install the CA on your client
   - Or enable "Trust self-signed certificates" in client settings
   - Check certificate isn't expired

### Connection Drops Frequently

**Symptoms:**
- Sync disconnects randomly
- "Connection lost" notifications
- Partial syncs

**Solutions:**

1. **Check network stability**
   - Test with other network traffic
   - Try wired connection instead of Wi-Fi

2. **Adjust timeout settings**
   - Client settings → Advanced → Connection timeout
   - Increase if on slow/unreliable network

3. **Check server logs**
   ```bash
   journalctl -u nosd -f
   ```

## Authentication Issues

### Device Token Expired

**Symptoms:**
- "Token expired" error
- Prompted to re-authenticate
- 401 errors in logs

**Solutions:**

1. **Client should auto-refresh** — Wait a moment and retry
2. **Manual re-authentication**
   - Open client settings
   - Sign out and sign back in
3. **Check device wasn't revoked**
   - Web UI → Settings → Sync → Devices
   - Verify your device is listed

### Too Many Devices

**Symptoms:**
- "Device limit reached" error when registering
- Cannot add new device

**Solutions:**

1. **Remove unused devices**
   - Web UI → Settings → Sync → Devices
   - Revoke devices you no longer use

2. **Default limit is 20 devices per user**

### Invalid Token

**Symptoms:**
- "Invalid token" or "Unauthorized" errors
- Sync stopped working

**Solutions:**

1. **Token may have been revoked**
   - Check if device still exists in web UI
   - Re-register device if needed

2. **Clock sync issue**
   - Ensure device time is accurate
   - Tokens are time-sensitive

## Sync Issues

### Files Not Syncing

**Symptoms:**
- New files don't appear on other devices
- Changes not propagating
- Stuck in "syncing" state

**Solutions:**

1. **Check sync status**
   - Look at client status icon/panel
   - Check for error messages

2. **Verify share has sync enabled**
   - Web UI → Shares → Edit share → Sync tab
   - Ensure "Enable NithronSync" is on

3. **Check file isn't excluded**
   - Review exclude patterns on share
   - Common exclusions: `*.tmp`, `~$*`, `.git/**`

4. **Check file size limits**
   - Shares can have max sync size
   - Very large files may be excluded

5. **Force rescan**
   - Client settings → Sync → Rescan local files
   - Or restart the sync client

### Conflicting Files

**Symptoms:**
- Files with "(conflict)" in name
- Duplicate files appearing
- Unexpected file versions

**Solutions:**

1. **Review conflict files**
   - Open both versions and compare
   - Keep the correct one, delete the other

2. **Understand conflict cause**
   - Same file edited on multiple devices while offline
   - Sync couldn't merge changes automatically

3. **Prevent conflicts**
   - Avoid editing same file on multiple devices simultaneously
   - Sync more frequently

### Sync Is Slow

**Symptoms:**
- Files take long to sync
- Progress bar moves slowly
- "Syncing..." for extended periods

**Solutions:**

1. **Check bandwidth limits**
   - Client settings → Sync → Bandwidth limit
   - Remove or increase limit

2. **Check network speed**
   ```bash
   # Test upload/download speed to server
   curl -o /dev/null -w "Download: %{speed_download}\n" https://server/dav/share/testfile
   ```

3. **Delta sync for large files**
   - Large file changes use delta sync automatically
   - Only changed blocks are transferred

4. **Check server load**
   - Web UI → Dashboard → System metrics
   - High CPU/disk may slow sync

5. **Too many small files**
   - Many small files = many requests
   - Consider archiving if possible

### Disk Space Issues

**Symptoms:**
- "Not enough disk space" error
- Sync paused due to space
- Files marked as "online only"

**Solutions:**

1. **Free up local disk space**
   - Empty trash/recycle bin
   - Remove unnecessary files

2. **Use selective sync**
   - Client settings → Selective Sync
   - Exclude large folders you don't need locally

3. **Check server space**
   - Web UI → Storage → Pools
   - Ensure server has available space

## Platform-Specific Issues

### Windows

**Antivirus blocking sync**
- Add NithronSync folder to exclusions
- Add `NithronSync.exe` to allowed apps

**Long path issues**
- Enable long paths in Windows
- Or use shorter folder names

**Explorer integration not working**
- Restart Explorer: `taskkill /f /im explorer.exe && explorer.exe`
- Reinstall client if persistent

### macOS

**Permission denied errors**
- System Preferences → Security → Privacy → Files and Folders
- Ensure NithronSync has access

**Finder extension not showing**
- System Preferences → Extensions → Finder Extensions
- Enable NithronSync

**Notarization warnings**
- Download from official source
- Right-click → Open to bypass Gatekeeper

### Linux

**Autostart not working**
```bash
# Check systemd service
systemctl --user status nithron-sync

# Enable autostart
systemctl --user enable nithron-sync
```

**Icon not appearing**
- Install libappindicator for tray icon
- Some desktop environments need extensions

**inotify limit reached**
```bash
# Increase inotify watches
echo "fs.inotify.max_user_watches=524288" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### iOS

**Background sync not working**
- Settings → NithronSync → Background App Refresh → On
- Keep app in memory (don't force-close)

**Files not showing in Files app**
- Settings → NithronSync → Show in Files → On
- Check file type is supported

### Android

**Battery optimization killing sync**
- Settings → Battery → NithronSync → Don't optimize

**Storage permission denied**
- Settings → Apps → NithronSync → Permissions → Storage

## Collecting Debug Information

### Enable Debug Logging

**Windows/macOS:**
1. Open client settings
2. Advanced → Debug logging → Enable
3. Reproduce the issue
4. Find logs in:
   - Windows: `%APPDATA%\NithronSync\logs\`
   - macOS: `~/Library/Logs/NithronSync/`

**Linux:**
```bash
nithron-sync --debug > ~/nithron-sync-debug.log 2>&1
```

### Server-Side Logs

```bash
# Sync handler logs
journalctl -u nosd | grep sync

# WebDAV logs
journalctl -u nosd | grep webdav

# Full nosd log
journalctl -u nosd -n 1000
```

### Creating a Bug Report

When reporting issues, include:

1. **Client version** (Settings → About)
2. **Server version** (Web UI footer)
3. **Platform** (Windows 11, Ubuntu 22.04, etc.)
4. **Steps to reproduce**
5. **Expected vs actual behavior**
6. **Log files** (with sensitive info redacted)
7. **Screenshots** if applicable

Submit issues at: https://github.com/Nithronverse/NithronOS/issues

## Getting Help

1. **Documentation** — [docs/sync/](./overview.md)
2. **Community Discord** — https://discord.gg/qzB37WS5AT
3. **GitHub Issues** — https://github.com/Nithronverse/NithronOS/issues

