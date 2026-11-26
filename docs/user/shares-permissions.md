# Shares & Permissions User Guide

## Overview

nOS provides a powerful and intuitive interface for managing network file shares. Whether you need to share files with Windows computers, set up Time Machine backups for Macs, or provide network storage for Linux systems, nOS makes it simple.

## Getting Started

### Accessing the Shares Interface

1. Log in to the nOS web interface
2. Click **Shares** in the sidebar
3. You'll see the shares dashboard showing all configured shares

![Shares Dashboard](../images/shares-dashboard.png)
*The shares dashboard shows all your network shares at a glance*

## Creating Your First Share

### Step 1: Click "Create Share"

Click the **+ Create Share** button in the top right corner of the shares page.

### Step 2: Configure Basic Settings

![Create Share Dialog - General Tab](../images/share-create-general.png)

1. **Share Name**: Enter a name for your share (e.g., "documents", "media", "backups")
   - Letters, numbers, hyphens and underscores, 1-32 characters
   - Can't be changed later (create a new share instead)

2. **Description**: Add an optional description to help identify the share's purpose

3. **Path Preview**: Shows where files will be stored (`/srv/shares/your-share-name`)

### Step 3: Configure Protocols

![Create Share Dialog - Protocols Tab](../images/share-create-protocols.png)

#### SMB/CIFS (Windows File Sharing)

Enable this for Windows computers and most devices:

- **Guest Access**: Allow anyone on the network to access without a password
  - ⚠️ Only enable for public/read-only content
  - Your administrator may disable this option via policy

- **Time Machine**: Enable Apple Time Machine backups
  - Perfect for backing up Mac computers
  - Requires sufficient storage space (2-3x Mac disk size recommended)

- **Recycle Bin**: Keep deleted files for recovery
  - Files go to a hidden `.recycle` folder
  - Manually clean up old files periodically

#### NFS (Unix/Linux File Sharing)

Enable this for Linux systems and advanced users:

- **Read Only**: Prevent modifications via NFS
- **Allowed Networks**: Restrict access to specific IP ranges
  - Default: Local network only (192.168.x.x, 10.x.x.x)
  - Add specific networks as needed

### Step 4: Set Permissions

![Create Share Dialog - Permissions Tab](../images/share-create-permissions.png)

Control who can access your share:

#### Owners (Read/Write Access)
- Can create, modify, and delete files
- Can create folders
- Full control over content

#### Readers (Read Only Access)
- Can view and download files
- Cannot modify or delete
- Cannot create new content

#### Everyone, or nobody

A new share without owners or readers is open to every nOS user (its owners
become `group:users`, the built-in group of all users). If you later remove
every owner and reader, the share is closed: nobody can open it until you add
someone.

#### SMB accounts

To open a share over SMB, a user needs an SMB password: an administrator sets
it under **Users** (API: `POST /api/v1/smb/users`). The SMB account has the
same name as the nOS user, so the share's permissions apply to it. Until then
the user is skipped (the share's apply status says so).

#### Adding Users and Groups

1. Click **Add Owner** or **Add Reader**
2. Search for users or groups
3. Select from the list
4. Users show with a person icon, groups with a people icon

![Permission Picker](../images/permission-picker.png)
*Search and select users or groups for permissions*

### Step 5: Review and Create

Click **Create Share** to save your configuration. The share will be immediately available on the network.

## Accessing Shares

### From Windows

1. Open File Explorer
2. In the address bar, type: `\\nos.local` or `\\<server-ip>`
3. Double-click your share
4. Enter credentials if prompted

![Windows Access](../images/windows-share-access.png)

### From macOS

1. Open Finder
2. Press `Cmd+K` or go to **Go → Connect to Server**
3. Enter: `smb://nos.local` or `smb://<server-ip>`
4. Select your share and click Connect

![macOS Access](../images/macos-share-access.png)

For Time Machine:
1. Open System Preferences → Time Machine
2. Click "Select Disk"
3. Choose your nOS Time Machine share
4. Enter credentials

### From Linux

#### GUI Method
Most file managers support network shares:
1. Open your file manager
2. Look for "Network" or "Other Locations"
3. Enter: `smb://nos.local` or use the server IP

#### Command Line (NFS)
```bash
# List available NFS exports
showmount -e nos.local

# Mount NFS share
sudo mount -t nfs nos.local:/srv/shares/documents /mnt/documents

# For permanent mounting, add to /etc/fstab:
nos.local:/srv/shares/documents /mnt/documents nfs defaults 0 0
```

## Managing Existing Shares

### Editing a Share

1. Click on a share name in the list
2. Modify settings as needed
3. Click **Save Changes**

Changes take effect immediately.

### Disabling Protocols

You can temporarily disable SMB or NFS without deleting the share:
1. Edit the share
2. Go to the Protocols tab
3. Toggle off SMB or NFS
4. Save changes

### Deleting a Share

1. Click the menu icon (⋮) next to a share
2. Select **Delete Share**
3. Confirm deletion

⚠️ **Important**: Deleting a share removes the configuration but does NOT delete the files. Files remain in `/srv/shares/<share-name>`.

## Features in Detail

### Guest Access

When enabled, anyone on your network can access the share without a password:
- Useful for media libraries or public documents
- Guests have read-only access by default
- Not recommended for sensitive data

**Security Note**: If your administrator has disabled guest access via policy, you'll see a warning badge.

### Time Machine Support

Perfect for Mac backups:
- Automatically discovered by macOS
- Supports multiple Macs (one share each recommended)
- Monitor disk usage - backups grow over time
- Consider setting quotas to prevent filling the disk

**Setup Tips**:
1. Create a dedicated share for each Mac
2. Name it clearly (e.g., "johns-macbook-tm")
3. Allocate 2-3x the Mac's disk size
4. Only give the Mac's user owner permissions

### Recycle Bin

Protects against accidental deletion:
- Deleted files move to `.recycle` folder
- Preserves folder structure
- Keeps multiple versions with timestamps
- **Not automatic cleanup** - manage manually

**Accessing the Recycle Bin**:
1. Enable "Show Hidden Files" in your file manager
2. Look for the `.recycle` folder
3. Restore files by moving them back
4. Permanently delete old files to free space

### Network Restrictions

Control access by network:
- SMB: Always restricted to local networks
- NFS: Can specify exact network ranges
- Default: RFC1918 private networks only
- Add specific subnets as needed

Example network ranges:
- `192.168.1.0/24` - Typical home network
- `10.0.0.0/8` - Large private network
- `172.16.0.0/12` - Medium private network

## Best Practices

### Naming Shares

Choose descriptive, memorable names:
- ✅ Good: `documents`, `media-library`, `backups-2024`
- ❌ Avoid: `share1`, `test`, `x`

### Security

1. **Use Strong Passwords**: For user accounts accessing shares
2. **Limit Guest Access**: Only for truly public content
3. **Regular Reviews**: Audit permissions periodically
4. **Network Isolation**: Use VLANs for sensitive shares

### Performance

1. **Separate Shares by Purpose**:
   - Documents (small files, frequent access)
   - Media (large files, streaming)
   - Backups (write-heavy, infrequent reads)

2. **Protocol Selection**:
   - SMB: Best for mixed environments
   - NFS: Better performance for Linux-only
   - Both: Maximum compatibility

3. **Monitor Usage**:
   - Check disk space regularly
   - Clean recycle bins periodically
   - Review access logs for unusual activity

### Organization

1. **Folder Structure**: Plan before creating shares
2. **Permissions**: Start restrictive, add access as needed
3. **Documentation**: Use descriptions to note share purposes
4. **Naming Convention**: Be consistent across shares

## Troubleshooting

### Cannot Access Share

1. **Check Network Connection**
   - Can you ping the server?
   - Is the server on the same network?

2. **Verify Credentials**
   - Username and password correct?
   - User has permissions on the share?

3. **Protocol Enabled?**
   - Is SMB/NFS enabled for the share?
   - Are services running? (Check Settings → System)

### Slow Performance

1. **Network Issues**
   - Check network speed
   - Look for packet loss
   - Consider upgrading to Gigabit

2. **Too Many Users**
   - Limit concurrent connections
   - Create read-only shares for popular content

3. **Large Files**
   - Use NFS for better large file performance
   - Consider protocol-specific tuning

### Permission Denied

1. **Check ACLs**: Verify user is in owners or readers list
2. **Group Membership**: Ensure user is in specified groups
3. **Guest Access**: May be disabled by policy
4. **File Permissions**: Some files may have additional restrictions

### Time Machine Not Working

1. **Discovery Issues**
   - Restart Avahi service
   - Check mDNS is not blocked
   - Try connecting directly by IP

2. **Space Issues**
   - Ensure adequate free space
   - Check quotas if configured
   - Clean old backups

3. **Compatibility**
   - Requires SMB2 or higher
   - Time Machine flag must be enabled
   - One backup destination per Mac

## Advanced Topics

### Using the API

Shares are under `/api/v1/shares` (signed in; changes need an administrator):

| Call | Does |
|---|---|
| `GET /api/v1/shares`, `GET /api/v1/shares/{name}` | list, read (by name or id) |
| `POST /api/v1/shares` | create `{name, description, smb, nfs, owners, readers}`; the path is `/srv/shares/<name>` |
| `PATCH /api/v1/shares/{name}` | change only the fields sent |
| `DELETE /api/v1/shares/{name}` | remove the share (the files stay) |
| `POST /api/v1/shares/{name}/test` | check `{config: {...}}` without saving: `{valid, errors}` |
| `GET /api/v1/shares/apply-status` | the result of the last apply (warnings such as users without SMB accounts) |
| `/api/v1/groups` | groups: `{name, description, members}`; a group used by a share can't be deleted |
| `/api/v1/policy` | `{"shares": {"guest_access_forbidden": true}}` |

```json
{
  "name": "media",
  "smb": {"enabled": true, "recycle": {"enabled": true}},
  "nfs": {"enabled": true, "networks": ["192.168.1.0/24"], "read_only": true},
  "owners": ["user:alice"],
  "readers": ["group:family"]
}
```

Principals are `user:<name>` and `group:<name>`; `group:users` is every user.
Each change is saved, then nos-agent writes `/etc/samba/smb.conf.d/nos-shares.conf`,
`/etc/exports.d/nos.exports`, the Unix groups (`nos-<group>`) and the folder's
ACL, and reloads Samba and the NFS exports. If that fails the change is kept
and the call answers `502 shares.apply_failed`; it applies with the next change
or when nosd restarts.

### Custom Access Patterns

For complex scenarios:

1. **Department Shares**: Use groups for team access
2. **Project Shares**: Time-limited with specific members
3. **Archive Shares**: Read-only with broad reader access
4. **Personal Shares**: Single owner, no readers

### Integration with Other Services

- **Backup Software**: Point to nOS shares
- **Media Servers**: Use shares as media libraries
- **Development**: Store code repositories
- **Virtualization**: ISO and VM storage

## Getting Help

- Check the [Admin Guide](../admin/shares.md) for technical details
- Visit our [GitHub Discussions](https://github.com/Nithron/nOS/discussions)
- Report issues on [GitHub](https://github.com/Nithron/nOS/issues)

## Quick Reference

### Share Naming Rules
- Use only letters, digits, hyphen (-) and underscore (_)
- Length: 1-32 characters
- Names differing only in case are the same share to Samba

### Default Paths
- Share location: `/srv/shares/<name>`
- Recycle bin: `/srv/shares/<name>/.recycle`
- Share list: `/etc/nos/shares.json`; groups: `/etc/nos/groups.json`
- Generated: `/etc/samba/smb.conf.d/nos-shares.conf`, `/etc/exports.d/nos.exports`

### Service Ports
- SMB: 445, 139
- NFS: 111, 2049
- mDNS: 5353

### Useful Commands
```bash
# Test SMB connection
smbclient -L //nos.local -U username

# List NFS exports
showmount -e nos.local

# Check share permissions
getfacl /srv/shares/sharename
```

---

*Last updated: November 2024 | nOS M2 Release*
