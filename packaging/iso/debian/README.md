# nOS ISO (Debian live-build)

## Requirements
- Debian/Ubuntu host (or container)
- live-build (`apt-get install -y live-build`)

## Place local .deb packages (optional)
Put built packages into `config/includes.chroot/root/debs/` or mount at runtime to install `nosd`, `nos-agent`, and `nos-web` during image build.

## Build
```bash
cd packaging/iso/debian
sudo ./auto/config
sudo lb build
```

The resulting ISO will be in the current directory.

Use `packaging/iso/build.sh` (it stages the .debs and generates the branding files from `VERSION`).

On first boot `nosd` shows the web UI address and a one-time setup code above the
login prompt (see `docs/admin/first-boot.md`).
