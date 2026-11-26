# nOS GRUB Branding and Theme

This directory contains the GRUB 2 theme and branding configuration for nOS ISO images.

## Overview

The nOS GRUB theme provides a branded boot experience for both BIOS and UEFI systems with:
- Custom dark theme matching nOS brand colors
- Branded menu entry titles
- Professional boot menu appearance
- Support for both legacy BIOS and modern UEFI boot

## Components

### Theme Files
Located in `includes.chroot/boot/grub/themes/nithron/`:
- `theme.txt` - GRUB theme configuration with nOS colors
- `background.png` - Dark background with centered nOS logo
- `DroidSans-32.pf2` - DroidSans font for menu items and text
- `DroidSans-Bold-32.pf2` - DroidSans Bold font for selected items and title

### Configuration Files
- `includes.chroot/etc/default/grub.d/99-nos.cfg` - GRUB defaults and theme activation
- `includes.binary/boot/grub/40_nos_titles.cfg` - Menu entry title overrides

### Build Hooks
Located in `hooks/normal/`:
- `030-grub-theme.hook.chroot` - Runs update-grub to apply theme
- `031-grub-titles.hook.chroot` - Updates menu entry titles to nOS branding
- `032-grub-binary-theme.hook.binary` - Copies theme to ISO boot partition

## Color Palette

Based on nOS brand colors (from assets/brand/palette.json):
- Background: `#0B0E13` (dark)
- Card/Panel: `#121622` 
- Text: `#E6EDF7` (light)
- Muted text: `#9AA7B8`
- Accent cyan: `#00D1FF`
- Accent blue: `#2D7FF9`
- Accent lime: `#A4F932`

## Generating Missing Assets

### Background Image
```bash
# Install ImageMagick if not present
apt-get install -y imagemagick

# Create dark background
convert -size 1024x768 xc:'#0B0E13' background.png

# Add logo (composite with existing logo file)
convert background.png ../../../../assets/brand/nos-logo-mark.png \
  -gravity center -resize 200x200 -composite background.png

# Optimize size (keep under 100KB)
convert background.png -quality 85 -strip background.png
```

### Font Files
The theme uses DroidSans fonts which are included:
- `DroidSans-32.pf2` - Regular font for menu items
- `DroidSans-Bold-32.pf2` - Bold font for selected items

To regenerate or create different sizes:
```bash
# Install grub tools
apt-get install -y grub-common

# Generate from TTF files (if you have DroidSans.ttf)
grub-mkfont -s 32 -o DroidSans-32.pf2 DroidSans.ttf
grub-mkfont -s 32 -o DroidSans-Bold-32.pf2 DroidSans-Bold.ttf
```

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

## Customization

To modify the theme:
1. Edit `theme.txt` for colors and layout
2. Replace `background.png` with custom artwork
3. Replace font files (`DroidSans-32.pf2`, `DroidSans-Bold-32.pf2`) with different fonts/sizes
4. Update `99-nos.cfg` for boot behavior changes

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

Theme configuration and hooks are part of nOS and follow the project license.
DejaVu fonts are freely available under their respective licenses.
