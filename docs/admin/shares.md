# nOS Network Shares Administration Guide

## Overview

nOS provides integrated network share management with support for SMB/CIFS and NFS protocols. The system includes advanced features like Time Machine support, recycle bins, and simple ACL management.

## Features

### Protocol Support
- **SMB/CIFS**: Windows file sharing with SMB2/SMB3
- **NFS**: Unix/Linux network filesystem (v3/v4)
- **mDNS/Bonjour**: Automatic discovery via Avahi

### Advanced Features
- **Time Machine**: Native macOS backup support
- **Recycle Bin**: Versioned file recovery
- **Guest Access**: Anonymous SMB access (optional)
- **POSIX ACLs**: Fine-grained permissions
- **Btrfs Subvolumes**: Automatic when available

## Creating Shares

### Via API

```bash
curl -X POST http://localhost:9000/api/v1/shares \
  -H "Content-Type: application/json" \
  -d '{
    "name": "documents",
    "smb": {
      "enabled": true,
      "guest": false,
      "recycle": {
        "enabled": true,
        "directory": ".recycle"
      }
    },
    "nfs": {
      "enabled": true,
      "networks": ["192.168.1.0/24"]
    },
    "owners": ["user:alice", "group:admins"],
    "readers": ["group:users"],
    "description": "Shared documents"
  }'
```

### Share Naming Rules
- Must start with lowercase letter or number
- Can contain: a-z, 0-9, hyphen (-), underscore (_)
- Length: 2-32 characters
- Examples: `documents`, `media-library`, `backup_2024`

## Permissions Model

### Access Levels

1. **Owners**: Full read/write/execute access
   - Can create, modify, delete files
   - Can create subdirectories
   - Applied via POSIX ACL with `rwx` permissions

2. **Readers**: Read and execute access
   - Can read files and list directories
   - Cannot modify or delete
   - Applied via POSIX ACL with `r-x` permissions

### Principal Format
- Users: `user:username`
- Groups: `group:groupname`

### Directory Structure
```
/srv/shares/
├── documents/          # Share root (mode: 02770, setgid)
│   ├── .recycle/      # Recycle bin (if enabled)
│   └── files...       # User data
```

## SMB/CIFS Configuration

### Guest Access
When `guest: true` is set:
- No authentication required
- Maps to `nobody` user
- Useful for public read-only shares
- Security consideration: LAN-only by default

### Recycle Bin
The recycle bin feature provides file recovery:
- Location: `.recycle` directory in share root
- Versioning: Keeps multiple versions with timestamps
- Tree structure: Preserves directory hierarchy
- Auto-cleanup: Configure via cron (not automatic)

Example recovery:
```bash
# List deleted files
ls -la /srv/shares/documents/.recycle/

# Restore a file
mv /srv/shares/documents/.recycle/important.doc_2024-01-15 \
   /srv/shares/documents/important.doc
```

### Time Machine Support

When `time_machine: true` is enabled:
- Share advertised via Bonjour/mDNS
- macOS automatically discovers the share
- Optimized for Time Machine backups
- Uses Samba vfs_fruit module

**Limitations:**
- One Time Machine share per Mac recommended
- Requires sufficient storage (2-3x Mac disk size)
- Set quota to prevent filling disk:
  ```bash
  # Set 1TB quota for Time Machine share
  btrfs qgroup limit 1T /srv/shares/timemachine
  ```

**macOS Client Setup:**
1. Open System Preferences → Time Machine
2. Click "Select Disk"
3. Choose the nOS share
4. Enter credentials if required

## NFS Configuration

### Network Access
NFS exports are restricted by network:
- Default: LAN networks only (RFC1918)
- Custom: Specify CIDR blocks
- Example: `["192.168.1.0/24", "10.0.0.0/8"]`

### Export Options
- **Read-only**: Set `read_only: true`
- **Root squash**: Enabled by default (security)
- **All squash**: Maps all users to nobody

### Client Mount
```bash
# List available exports
showmount -e nos.local

# Mount NFS share
sudo mount -t nfs nos.local:/srv/shares/documents /mnt/documents

# Persistent mount (add to /etc/fstab)
nos.local:/srv/shares/documents /mnt/documents nfs defaults 0 0
```

## Service Management

### Health Checks
```bash
# Test Samba configuration
testparm -s

# Check NFS exports
exportfs -v

# Verify services
systemctl status smbd nfs-server avahi-daemon
```

### Reload Services
Services are automatically reloaded when shares change:
- Samba: `systemctl reload smbd`
- NFS: `exportfs -ra && systemctl reload nfs-server`
- Avahi: `systemctl reload avahi-daemon`

## Firewall Rules

Required ports (LAN-only by default):
- **445/tcp**: SMB/CIFS
- **139/tcp**: NetBIOS (legacy SMB)
- **111/tcp,udp**: RPC portmapper
- **2049/tcp,udp**: NFS
- **5353/udp**: mDNS/Bonjour

## Troubleshooting

### SMB Issues

**Cannot connect to share:**
```bash
# Check Samba status
systemctl status smbd
testparm -s

# Check logs
journalctl -u smbd -n 50
tail -f /var/log/samba/log.smbd

# Test authentication
smbclient -L localhost -U username
```

**Permission denied:**
```bash
# Check ACLs
getfacl /srv/shares/sharename

# Verify user groups
id username

# Check Samba user
pdbedit -L
```

### NFS Issues

**Mount fails:**
```bash
# Check exports
exportfs -v

# Test from client
rpcinfo -p nos.local
showmount -e nos.local

# Check logs
journalctl -u nfs-server -n 50
```

**Permission issues:**
- Verify network is in allowed list
- Check if root_squash is affecting access
- Ensure UIDs match between client and server

### Time Machine Issues

**Share not visible on Mac:**
```bash
# Check Avahi service
systemctl status avahi-daemon
avahi-browse -a -t

# Verify service file
cat /etc/avahi/services/nos-tm.service

# Restart discovery
systemctl restart avahi-daemon
```

**Backup fails:**
- Check available space
- Verify SMB version (requires SMB2+)
- Check vfs_fruit module loaded
- Review Samba logs for errors

## Best Practices

### Security
1. Use strong passwords for SMB users
2. Limit guest access to read-only shares
3. Restrict NFS to specific networks
4. Regular ACL audits
5. Monitor access logs

### Performance
1. Use Btrfs subvolumes for shares when possible
2. Enable SMB3 multichannel for better throughput
3. Consider separate shares for different workloads
4. Set appropriate oplocks/leases settings

### Maintenance
1. Regular recycle bin cleanup:
   ```bash
   # Delete files older than 30 days
   find /srv/shares/*/.recycle -type f -mtime +30 -delete
   ```

2. Monitor disk usage:
   ```bash
   df -h /srv/shares
   du -sh /srv/shares/*
   ```

3. Backup share configurations:
   ```bash
   cp /etc/nos/shares.json /backup/
   tar -czf /backup/share-configs.tar.gz \
     /etc/samba/smb.conf.d/ \
     /etc/exports.d/
   ```

## API Reference

### Endpoints
- `GET /api/shares` - List all shares
- `POST /api/shares` - Create new share
- `PATCH /api/shares/{name}` - Update share
- `DELETE /api/shares/{name}` - Delete share
- `POST /api/shares/{name}/test` - Validate configuration

### Example: Update Share
```bash
curl -X PATCH http://localhost:9000/api/v1/shares/documents \
  -H "Content-Type: application/json" \
  -d '{
    "smb": {
      "enabled": true,
      "guest": true
    }
  }'
```

### Example: Test Configuration
```bash
curl -X POST http://localhost:9000/api/v1/shares/documents/test \
  -H "Content-Type: application/json" \
  -d '{
    "config": {
      "nfs": {
        "enabled": true,
        "read_only": true
      }
    }
  }'
```

## Limitations

### Share Limits
- Maximum share name length: 32 characters
- Maximum path length: 255 characters
- Maximum ACL entries: System dependent (typically 32)

### Time Machine Limits
- One backup destination per Mac recommended
- No built-in quota management (use filesystem quotas)
- Requires case-insensitive filesystem or compatibility mode

### Protocol Limits
- SMB: 16TB file size limit
- NFS v3: 2GB file size limit (use v4 for larger files)
- Guest access: SMB only (NFS uses UID mapping)
