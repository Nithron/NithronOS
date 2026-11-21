# NithronOS Installer Guide

## Overview

NithronOS provides a guided installer (`nos-installer`) that creates a fully configured system with Btrfs subvolumes, proper mount options, and all necessary system components.

## Requirements

- **Disk Space**: Minimum 20 GB, recommended 50 GB or more
- **Memory**: Minimum 2 GB RAM, recommended 4 GB or more
- **Boot Mode**: UEFI (Legacy BIOS not supported)
- **Network**: Internet connection for package downloads (optional if using offline installer)

## Installation Process

### 1. Boot the Live ISO

Boot the NithronOS live ISO on your target system. The installer will be available from the console.

### 2. Run the Installer

```bash
sudo nos-installer
```

The installer must be run as root to perform system operations.

### 3. Installation Steps

#### Step 1: Disk Selection

- The installer will detect all available disks
- Select the target disk for installation
- **WARNING**: All data on the selected disk will be destroyed

#### Step 2: Confirmation

- Confirm the destructive operation
- Type `DESTROY` to proceed with installation

#### Step 3: Disk Partitioning

The installer creates a GPT partition table with:
- **ESP (EFI System Partition)**: 512 MiB, FAT32
- **Root Partition**: Remaining space, Btrfs

#### Step 4: Btrfs Layout

The following subvolumes are created automatically:

| Subvolume | Mount Point | Purpose |
|-----------|------------|---------|
| @ | / | Root filesystem |
| @home | /home | User home directories |
| @var | /var | Variable data |
| @log | /var/log | System logs |
| @snapshots | /snapshots | Snapshot storage |

**Mount Options**:
- Base: `defaults,noatime,compress=zstd:3`
- SSD additions: `ssd,discard=async`

#### Step 5: System Bootstrap

The installer will:
1. Copy base system from live image or use debootstrap
2. Install kernel and bootloader
3. Install NithronOS packages (nosd, nos-agent, nos-web, caddy)
4. Configure networking and services

#### Step 6: Bootloader Installation

GRUB is installed with:
- EFI boot support
- NithronOS branding (if available)
- Proper Btrfs subvolume configuration

#### Step 7: System Configuration

- `/etc/fstab` generation with UUID-based mounts
- Hostname configuration (default: `nithronos`)
- Timezone setup (default: `UTC`)
- Service user creation
- Service enablement

#### Step 8: Finalization

- Update initramfs
- Copy installation log to target system
- Unmount filesystems

## Post-Installation

### First Boot

After installation and reboot:

1. **Setup code**: a one-time 6-digit setup code and the address to open are shown above the login prompt
2. **Web Setup**: Navigate to `https://<server-ip>` in a web browser
3. **Setup Wizard**: Complete the first-boot setup wizard using the setup code

### First-Boot Setup Wizard

The web-based setup wizard includes:

1. **Setup code**: Enter the 6-digit code from the server screen
2. **Admin Account**: Create the administrator account
3. **System Configuration**: Set hostname, timezone, and NTP
4. **Network Configuration**: Configure network interfaces (DHCP or static)
5. **Telemetry**: Opt-in for anonymous usage statistics
6. **Two-Factor Authentication**: Optional TOTP setup

## Customization

### Custom Mount Options

To modify Btrfs mount options after installation:

1. Edit `/etc/fstab`
2. Update mount options for each subvolume
3. Remount or reboot

Example:
```
UUID=xxx / btrfs defaults,noatime,compress=zstd:3,subvol=@ 0 1
```

### Additional Subvolumes

To create additional subvolumes:

```bash
# Mount the root Btrfs filesystem
mount /dev/sdX2 /mnt

# Create new subvolume
btrfs subvolume create /mnt/@docker

# Add to /etc/fstab
echo "UUID=xxx /var/lib/docker btrfs defaults,noatime,compress=zstd:3,subvol=@docker 0 2" >> /etc/fstab

# Create mount point and mount
mkdir -p /var/lib/docker
mount /var/lib/docker
```

## Troubleshooting

### Installation Fails

- **Check logs**: `/var/log/nithronos-installer.log`
- **Verify disk health**: `smartctl -a /dev/sdX`
- **Check available space**: Ensure at least 20 GB free
- **Network issues**: Check connectivity if packages fail to download

### First-Boot Issues

#### Setup Code Not Displayed

- Read it from the log: `sudo journalctl -u nosd | grep 'setup code' | tail -n 1`
- Check the services: `systemctl status nosd nos-issue-reload.path`
- Check the banner file: `cat /run/nos/firstboot.issue`

#### Cannot Access Web Interface

- Verify Caddy is running: `systemctl status caddy`
- Check firewall: `nft list ruleset`
- Test locally: `curl -k https://localhost`

#### Setup Wizard Errors

- Backend not reachable: Check `nosd` service
- Code expired: use the new code shown on the screen (codes are replaced automatically)
- Permission errors: Verify `/etc/nos` permissions

### Secure Boot

NithronOS currently does not support Secure Boot. If you encounter boot issues:

1. Disable Secure Boot in UEFI/BIOS settings
2. Ensure CSM/Legacy mode is disabled
3. Boot in UEFI mode only

## Advanced Installation

### Unattended Installation

For automated deployments, create an answer file:

```bash
nos-installer --unattended --config install.yaml
```

Example `install.yaml`:
```yaml
disk: /dev/sda
hostname: nas01
timezone: America/New_York
network:
  interface: eth0
  dhcp: true
```

### Custom Package Selection

To include additional packages during installation:

```bash
nos-installer --packages "htop,vim,tmux"
```

### Installation from Network

PXE boot support is planned for future releases.

## Recovery

If the system becomes unbootable:

1. Boot from live ISO
2. Mount the root subvolume: `mount -o subvol=@ /dev/sdX2 /mnt`
3. Chroot into the system: `chroot /mnt`
4. Repair as needed
5. Update GRUB: `update-grub`
6. Exit and reboot

## See Also

- [First-Boot Setup](first-boot.md)
- [Storage Configuration](../storage/pools.md)
- [System Recovery](../admin/recovery.md)
