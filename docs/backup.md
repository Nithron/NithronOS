# Backup & Replication Guide

## Overview

nOS provides comprehensive backup and replication capabilities using Btrfs snapshots. The system supports scheduled snapshots with retention policies, replication to remote destinations, and both full and file-level restore operations.

## Key Features

- **Instant Snapshots**: Near-instantaneous Btrfs snapshots with minimal storage overhead
- **GFS Retention**: Grandfather-Father-Son retention policies for efficient long-term storage
- **Remote Replication**: SSH and cloud (rclone) replication support
- **Flexible Restore**: Full subvolume or file-level restore options
- **Scheduled Backups**: Cron-based scheduling with pre/post hooks
- **Job Management**: Track and monitor backup operations

## Snapshot Management

### Creating Manual Snapshots

Snapshots can be created manually through the web UI or API:

```bash
# Via API
curl -X POST https://localhost/api/v1/backup/snapshots/create \
  -H "Content-Type: application/json" \
  -d '{
    "subvolumes": ["@", "@home"],
    "tag": "before-upgrade"
  }'
```

### Snapshot Storage

Snapshots are stored under the `@snapshots` subvolume:

```
@snapshots/
├── @/
│   ├── 20240101-120000
│   └── 20240102-120000-before-upgrade
├── @home/
│   └── 20240101-120000
└── restore-safety/
    └── @-20240103-140000
```

### Snapshot Properties

- **Read-only**: All snapshots are created as read-only to prevent accidental modification
- **Incremental**: Only changed blocks are stored, minimizing storage usage
- **Atomic**: Snapshots are atomic and consistent at the filesystem level

## Backup Schedules

### Creating a Schedule

Schedules define when and what to backup:

1. Navigate to **Backup → Schedules** in the web UI
2. Click **Create Schedule**
3. Configure:
   - **Name**: Descriptive name for the schedule
   - **Subvolumes**: Which subvolumes to backup
   - **Frequency**: Hourly, daily, weekly, monthly, or custom cron
   - **Retention**: How many snapshots to keep

### Frequency Options

| Type | Configuration | Example |
|------|--------------|---------|
| Hourly | Minute of hour | Every hour at :30 |
| Daily | Time of day | Daily at 02:00 |
| Weekly | Day and time | Sundays at 03:00 |
| Monthly | Day of month and time | 1st of month at 04:00 |
| Cron | Cron expression | `0 */6 * * *` (every 6 hours) |

### GFS Retention Policy

The Grandfather-Father-Son retention policy optimizes storage by keeping:

- **Daily snapshots**: Recent backups for quick recovery
- **Weekly snapshots**: One per week for medium-term retention
- **Monthly snapshots**: One per month for longer-term
- **Yearly snapshots**: One per year for compliance/archival

Example retention policy:
- Minimum to keep: 3
- Daily: 7 (last week)
- Weekly: 4 (last month)
- Monthly: 6 (last 6 months)
- Yearly: 2 (last 2 years)

### Pre/Post Hooks

Hooks allow running commands before/after snapshots:

**Pre-hooks** (run before snapshot):
- Stop databases for consistency
- Flush application caches
- Create application-level backups

**Post-hooks** (run after snapshot):
- Restart services
- Send notifications
- Trigger replication

## Replication

### SSH Replication

Replicate snapshots to remote servers via SSH:

1. **Add Destination**:
   ```json
   {
     "name": "Remote Backup Server",
     "type": "ssh",
     "host": "backup.example.com",
     "port": 22,
     "user": "backup",
     "path": "/backups/nos",
     "bandwidth_limit": 10000
   }
   ```

2. **Store SSH Key**:
   ```bash
   curl -X POST https://localhost/api/v1/backup/destinations/{id}/key \
     -H "Content-Type: application/json" \
     -d '{"key": "-----BEGIN RSA PRIVATE KEY-----\n..."}'
   ```

3. **Test Connection**:
   ```bash
   curl -X POST https://localhost/api/v1/backup/destinations/{id}/test
   ```

4. **Start Replication**:
   ```bash
   curl -X POST https://localhost/api/v1/backup/replicate \
     -H "Content-Type: application/json" \
     -d '{
       "destination_id": "dest-123",
       "snapshot_id": "snap-456",
       "base_snapshot_id": "snap-455"
     }'
   ```

### Rclone Cloud Sync

Sync snapshots to cloud storage providers:

1. **Configure Rclone Remote**:
   ```bash
   rclone config
   # Follow prompts to configure S3, B2, Google Drive, etc.
   ```

2. **Add Rclone Destination**:
   ```json
   {
     "name": "S3 Backup",
     "type": "rclone",
     "remote_name": "s3-backup",
     "remote_path": "bucket/nos-backups",
     "bandwidth_limit": 5000,
     "concurrency": 4
   }
   ```

### Incremental Replication

Incremental sends transfer only changed blocks:

- **Initial**: Full snapshot transfer
- **Subsequent**: Only differences from parent snapshot
- **Benefits**: Reduced bandwidth and faster transfers

## Restore Operations

### Restore Types

#### Full Subvolume Restore

Replaces entire subvolume with snapshot:

- **Use when**: System corruption, major rollback needed
- **Downtime**: Requires stopping affected services
- **Process**:
  1. Safety snapshot created
  2. Services stopped
  3. Subvolume replaced atomically
  4. Services restarted

#### File-Level Restore

Copies specific files from snapshot:

- **Use when**: Recovering deleted/modified files
- **Downtime**: None
- **Process**:
  1. Snapshot mounted read-only
  2. Files copied to target
  3. Permissions preserved
  4. Snapshot unmounted

### Creating a Restore Plan

Before executing a restore, create a plan to review:

```bash
curl -X POST https://localhost/api/v1/backup/restore/plan \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "local",
    "source_id": "snap-123",
    "restore_type": "files",
    "target_path": "/home/user/recovered"
  }'
```

The plan shows:
- Actions to be performed
- Services that will be stopped
- Estimated time
- Required permissions

### Restore from Remote

Restore from SSH destination:

```bash
curl -X POST https://localhost/api/v1/backup/restore/apply \
  -H "Content-Type: application/json" \
  -d '{
    "source_type": "ssh",
    "source_id": "destination-id:snapshot-name",
    "restore_type": "full",
    "target_path": "/"
  }'
```

## Job Management

### Job States

| State | Description |
|-------|------------|
| pending | Job queued but not started |
| running | Job actively executing |
| succeeded | Job completed successfully |
| failed | Job failed with error |
| canceled | Job canceled by user |

### Monitoring Jobs

View running and recent jobs:

```bash
# List recent jobs
curl https://localhost/api/v1/backup/jobs?limit=10

# Get specific job
curl https://localhost/api/v1/backup/jobs/{job-id}

# Cancel running job
curl -X POST https://localhost/api/v1/backup/jobs/{job-id}/cancel
```

### Job Progress

Jobs report progress for long-running operations:
- **Progress**: Percentage complete (0-100)
- **Bytes transferred**: For replication jobs
- **Log entries**: Detailed operation logs
- **ETA**: Estimated time remaining

## Best Practices

### Backup Strategy

1. **System Subvolumes** (`@`, `@var`):
   - Daily snapshots
   - Keep 7 days, 4 weeks, 6 months

2. **User Data** (`@home`):
   - Hourly during business hours
   - Daily after hours
   - Keep 24 hourly, 30 daily, 12 monthly

3. **Application Data** (`/srv/apps`):
   - Before and after updates
   - Daily with app-specific hooks
   - Coordinate with app maintenance windows

### Testing Restores

Regularly test restore procedures:

1. **Monthly**: File-level restore test
2. **Quarterly**: Full subvolume restore to test system
3. **Annually**: Complete disaster recovery drill

### Security

- **Encryption**: Use SSH for remote transfers
- **Key Management**: Store SSH keys securely
- **Access Control**: Limit restore permissions to admins
- **Audit**: Log all backup/restore operations

### Monitoring

Set up alerts for:
- Failed backup jobs
- Missed scheduled backups
- Low disk space for snapshots
- Replication lag
- Retention policy violations

## Troubleshooting

### Snapshot Creation Fails

**Error: "No space left on device"**
- Check available space: `btrfs filesystem df /`
- Clean old snapshots: Review retention policies
- Balance filesystem: `btrfs balance start -dusage=50 /`

**Error: "Read-only filesystem"**
- Check filesystem errors: `btrfs check /dev/sdX`
- Review system logs: `journalctl -xe`

### Replication Issues

**SSH Connection Failed**
- Verify SSH key permissions (600)
- Test manual SSH connection
- Check firewall rules
- Verify remote path exists

**Slow Transfer Speed**
- Check bandwidth limits
- Monitor network usage
- Consider incremental sends
- Optimize compression settings

### Restore Problems

**Service Won't Start After Restore**
- Check service logs
- Verify file permissions
- Ensure all subvolumes mounted
- Review configuration files

**Restore Takes Too Long**
- Use file-level restore for small changes
- Schedule during maintenance window
- Consider partial restore
- Optimize network for remote restores

## Advanced Topics

### Custom Retention Scripts

Create custom retention policies:

```python
#!/usr/bin/env python3
# Custom retention: Keep snapshots from specific dates

import json
import subprocess
from datetime import datetime, timedelta

def should_keep(snapshot_date):
    # Keep first of month
    if snapshot_date.day == 1:
        return True
    # Keep if Friday
    if snapshot_date.weekday() == 4:
        return True
    # Keep if less than 7 days old
    if (datetime.now() - snapshot_date).days < 7:
        return True
    return False

# Apply custom retention logic
snapshots = json.loads(subprocess.check_output(['nosd', 'snapshots', 'list']))
for snap in snapshots:
    if not should_keep(datetime.fromisoformat(snap['created_at'])):
        subprocess.run(['nosd', 'snapshot', 'delete', snap['id']])
```

### Btrfs Send Stream Format

Understanding the send stream for custom tools:

```bash
# Create send stream
btrfs send @snapshots/@/20240101-120000 > snapshot.send

# Examine stream
btrfs receive --dump < snapshot.send

# Stream to remote with compression
btrfs send @snapshots/@/20240101-120000 | \
  zstd -T0 | \
  ssh backup@remote "zstd -d | btrfs receive /backups"
```

### Snapshot Diff

Compare snapshots to see changes:

```bash
# Find changed files
btrfs send -p @snapshots/@/20240101-120000 @snapshots/@/20240102-120000 | \
  btrfs receive --dump | grep ^update_extent

# Get actual diff
diff -r @snapshots/@/20240101-120000 @snapshots/@/20240102-120000
```

## API Reference

See the [API Documentation](api/backup.yaml) for complete endpoint reference.

## See Also

- [Storage Pools](storage/pools.md)
- [Disaster Recovery](admin/recovery.md)
- [System Updates](updates.md)
