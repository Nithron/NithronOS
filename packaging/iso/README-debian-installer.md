# nOS installer

The ISO has two parts:

- **nOS Live** (`/live`): the full system, including the nOS packages (nosd, nos-agent, nos-web, ...).
- **Install nOS** (`/install`): live-build's Debian installer (`--debian-installer live`). It copies the
  live system to the disk with `live-installer`, so nothing is downloaded during the install.

`packaging/iso/build.sh` builds both.

## How the nOS packages get onto the disk

`build.sh` copies the `.deb`s to `config/packages.chroot/` and lists them in
`config/package-lists/nos-local.list.chroot` (both generated, git-ignored). live-build then installs them
like any other package. The post-build check fails the build if any of them is missing from
`live/filesystem.packages` or appears in `live/filesystem.packages-remove` (the list of live-only packages the
installer removes from the installed system).

## Installer configuration

| File | Purpose |
|---|---|
| `debian/config/includes.installer/preseed.cfg` | Embedded in `/install/initrd.gz` and loaded automatically. |
| `debian/config/includes.installer/usr/lib/nos-installer/late-command.sh` | Runs at the end of the install: checks that nosd, nos-agent and the web UI are on the disk (stops with an error otherwise), writes the APT sources (Debian, updates, security), enables the services, adds the console administrator to `sudo`. |
| `debian/config/includes.binary/isolinux/isolinux.cfg` | BIOS boot menu. |
| `debian/config/includes.binary/boot/grub/grub.cfg` | UEFI boot menu. |

The install entries boot `/install/vmlinuz` with `auto=true priority=high`: every preseeded answer is used
silently and the installer only asks for

1. the password of the console administrator `nosadmin` (it has `sudo`; there is no root password),
2. the disk to install to, when the machine has more than one,
3. confirmation before that disk is erased.

Partitioning: GPT with a BIOS boot partition, a 512 MB EFI system partition, swap (2 to 4 GB) and Btrfs for `/`
(`noatime,compress=zstd:3`), so the same disk boots on BIOS and UEFI.

After the first boot, the console shows the web UI address and a one-time setup code (see
`docs/admin/first-boot.md`).

## Testing an installer change without rebuilding the ISO

The preseed and `late-command.sh` live in the installer initrd, so they can be swapped into an existing ISO's
`/install/initrd.gz` (extract with `bsdtar`, replace the files, repack as a `newc` cpio archive owned by root) and
booted with QEMU's `-kernel /install/vmlinuz -initrd <repacked> -append "auto=true priority=high DEBIAN_FRONTEND=text ---"`
next to `-cdrom <iso>`. Add `log_host=10.0.2.2 log_port=<port>` to receive the installer's syslog over UDP on the host.

## Known limitations

- Secure Boot: the ISO isn't shim-signed yet; disable Secure Boot to boot it.
