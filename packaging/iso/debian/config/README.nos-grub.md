# nOS GRUB Branding and Theme

This directory contains the GRUB 2 theme and branding configuration for nOS ISO images.

## Overview

The boot menu follows the Nithron identity (DESIGN-APPS.md in the
nithron-site repository, "Other surfaces of a product"): the ground and the
product's name in type, in two colours, the tokens' dark `--page` (#0c0d0f)
and `--ink` (#eceef0). No drawn logo, no accent colour, no progress bar. The
same theme serves the ISO (BIOS and UEFI) and the installed system.

## Components

### Theme files
In `includes.chroot/boot/grub/themes/nithron/` and the same in
`includes.binary/boot/grub/themes/nithron/`:
- `theme.txt`: the layout; every entry in ink, the chosen one in a 1 px ink
  frame, the timeout and the keys as words.
- `background.png`: the ground with "nOS" in the Nithron Pixel face at 64 px.
- `highlight_*.png`: the pieces of the frame round the chosen entry.

Text uses GRUB's Unifont (`unicode.pf2`, a bitmap face), loaded by
`includes.binary/boot/grub/grub.cfg` and copied by hook 032.

Also made from the same face: `includes.binary/isolinux/splash.png` and the
installer's banner `includes.installer/usr/share/graphics/logo_debian.png`.

### Configuration files
- `includes.chroot/etc/default/grub.d/99-nos.cfg` - GRUB defaults and theme activation
- `includes.binary/boot/grub/40_nos_titles.cfg` - Menu entry title overrides

### Build hooks
In `hooks/normal/`:
- `030-grub-theme.hook.chroot` - Runs update-grub to apply theme
- `031-grub-titles.hook.chroot` - Updates menu entry titles to nOS branding
- `032-grub-binary-theme.binary` - Copies theme and font to the ISO

## Making the pictures

```bash
python3 packaging/iso/scripts/gen-boot-art.py \
  <nithron-site>/packages/tokens/fonts/nithron-pixel.ttf packaging/iso/debian/config
```

It draws without smoothing and fails unless each picture holds exactly two
colours. It needs Pillow.

## Menu Entries

The branded GRUB menu will display:
1. **nOS Live (amd64)** - Main live boot option
2. **Install nOS** - Debian installer (if included)
3. **nOS Live (safe graphics)** - Fallback graphics mode
4. **Advanced options for nOS** - Submenu with:
   - nOS Live (failsafe)
   - Memory test (if memtest86+ included)

## Verification

After building the ISO, boot it in a VM and verify:

```bash
# Check theme is configured
grep -R "GRUB_THEME" /etc/default/grub*

# Verify theme file exists
test -f /boot/grub/themes/nithron/theme.txt && echo "Theme found"

# Check branded menu entries
grep -R "nOS Live (amd64)" /boot/grub/grub.cfg
```

## Build Process

During live-build ISO creation:
1. Theme files are placed in the chroot at `/boot/grub/themes/nithron/`
2. GRUB defaults are configured via `/etc/default/grub.d/99-nos.cfg`
3. Hook 030 runs `update-grub` to generate initial config
4. Hook 031 updates all menu entry titles to nOS branding
5. Hook 032 copies theme to binary boot partition for ISO
6. Both BIOS and UEFI boot modes use the same theme

## Troubleshooting

### Theme Not Showing
- Ensure `GRUB_TERMINAL_OUTPUT="gfxterm"` is set
- Verify theme files exist in `/boot/grub/themes/nithron/`
- Check GRUB_THEME path in `/etc/default/grub.d/99-nos.cfg`

### Wrong Menu Titles
- Check hook execution order (030, 031, 032)
- Verify sed patterns in `031-grub-titles.hook.chroot`
- Look for grub.cfg in multiple locations (BIOS vs UEFI)

### UEFI vs BIOS Differences
- UEFI uses `/boot/efi/EFI/debian/grub.cfg`
- BIOS uses `/boot/grub/grub.cfg`
- Theme should be copied to both locations by hooks

## License

Theme configuration and hooks are part of nOS and follow its licence. The
Nithron Pixel face is from @nithron/tokens 0.4.0 (Nithron Community License
v1.0); Unifont comes with GRUB.
