# nOS Installer Guide

## Overview

The nOS ISO boots into a menu with **nOS Live** and **Install nOS**. The installer is the Debian
installer, preconfigured for nOS: it copies the complete nOS system from the ISO to the disk (no
downloads needed) and asks only what a person has to decide.

## Requirements

- **Disk**: one disk for the system, at least 20 GB (50 GB or more recommended). Keep your data disks separate;
  they are set up later from the web UI.
- **Memory**: at least 2 GB RAM, 4 GB or more recommended.
- **Firmware**: UEFI or legacy BIOS. Secure Boot must be disabled for now (the ISO isn't shim-signed yet).
- **Network**: optional during the install; DHCP is used when available.

## Installing

1. Boot the ISO and choose **Install nOS**. Use **Install nOS (Serial console @ttyS0)** for machines
   without a screen, and **Safe graphics** if the screen stays blank.
2. **Console password**: choose the password for `nosadmin`, the console/SSH administrator account (it has `sudo`;
   there is no root password). This is not the web UI login; that account is created in the setup wizard.
3. **Disk**: if the machine has more than one disk, choose the system disk. Check the size and model carefully:
   device names such as `sda` and `sdb` can change between boots.
4. **Confirm**: the installer lists the partitions it will write and asks before erasing the disk (default: No).
5. The installer copies the system, installs the boot loader on the disk you chose, and restarts.

### Disk layout

GPT, so the same disk boots on UEFI and BIOS machines:

| Partition | Size | Use |
|---|---|---|
| BIOS boot | 1 MB | GRUB on BIOS machines |
| EFI system partition | 512 MB | GRUB on UEFI machines |
| swap | 2 to 4 GB | |
| Btrfs | rest of the disk | `/`, mounted with `noatime,compress=zstd:3` |

### What the installer sets up

- The nOS services (`nosd`, `nos-agent`, `caddy`, the web UI) are installed and enabled; the install stops
  with an error if they are missing from the ISO.
- APT sources for Debian, its updates and **security updates**.
- Hostname `nos`, time zone UTC (both can be changed in the setup wizard).

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
UUID=xxx / btrfs noatime,compress=zstd:3,subvol=@rootfs 0 0
```

### Additional Subvolumes

To create additional subvolumes:

```bash
# Mount the root Btrfs filesystem
mount /dev/sdX4 /mnt

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

- **Installer log**: press Alt+F4 during the install, or read `/var/log/installer/syslog` on the installed system
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

nOS currently does not support Secure Boot. If you encounter boot issues:

1. Disable Secure Boot in the UEFI settings
2. Boot in UEFI mode, or in legacy/CSM mode; both are supported

## Advanced Installation

### Unattended Installation

The installer reads its answers from `config/includes.installer/preseed.cfg` (embedded in the ISO). For a fully
unattended install, also answer the three questions it leaves open, for example in a preseed served over HTTP and
passed on the installer's boot line as `url=http://<server>/nos.cfg`:

```
d-i passwd/user-password-crypted password <hash from: mkpasswd -m sha-512>
d-i partman-auto/disk string /dev/disk/by-id/<the system disk>
d-i partman/confirm_nooverwrite boolean true
d-i partman/confirm boolean true
```

Use a `/dev/disk/by-id/...` path: names such as `/dev/sda` can point at a different disk on the next boot.

### Installation from Network

PXE boot support is planned for future releases.

## Recovery

If the system becomes unbootable:

1. Boot from live ISO
2. Mount the root subvolume: `mount -o subvol=@rootfs /dev/sdX4 /mnt`
3. Chroot into the system: `chroot /mnt`
4. Repair as needed
5. Update GRUB: `update-grub`
6. Exit and reboot

## See Also

- [First-Boot Setup](first-boot.md)
- [Storage Configuration](../storage/pools.md)
- [System Recovery](../admin/recovery.md)
