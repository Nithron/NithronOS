#!/usr/bin/env python3
"""The nOS boot and installer pictures on the Nithron identity
(DESIGN-APPS.md, "Other surfaces of a product": the ground and the
product's name in type, two colours at most).

Writes, from the Nithron Pixel face (@nithron/tokens 0.4.0), with no
smoothing, so every picture holds exactly the dark ground and the ink:
  background.png      1024x768, the GRUB menu (ISO and installed system)
  splash.png          1024x768, the same for ISOLINUX
  logo_debian.png     800x75, the installer's banner
  highlight_*.png     1x1 pieces of a 1 px ink frame round the chosen entry

usage: gen-boot-art.py <nithron-pixel.ttf> <packaging/iso/debian/config>
"""
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

# The tokens' dark theme: --page and --ink.
GROUND = (0x0C, 0x0D, 0x0F)
INK = (0xEC, 0xEE, 0xF0)


def name_on_ground(font_path: str, size: tuple[int, int], px: int, at: tuple[int, int]) -> Image.Image:
    im = Image.new("RGB", size, GROUND)
    draw = ImageDraw.Draw(im)
    draw.fontmode = "1"  # no anti-aliasing: whole pixels, two colours
    draw.text(at, "nOS", font=ImageFont.truetype(font_path, px), fill=INK)
    colours = im.getcolors()
    assert colours is not None and len(colours) == 2, colours
    return im


def main() -> None:
    font, config = sys.argv[1], Path(sys.argv[2])
    # The name at 64 px (8 x 8), level with the menu's left edge (15 %).
    screen = name_on_ground(font, (1024, 768), 64, (152, 80))
    for theme in ("includes.chroot", "includes.binary"):
        d = config / theme / "boot/grub/themes/nithron"
        d.mkdir(parents=True, exist_ok=True)
        screen.save(d / "background.png", optimize=True)
        for part in ("nw", "n", "ne", "w", "e", "sw", "s", "se"):
            Image.new("RGB", (1, 1), INK).save(d / f"highlight_{part}.png")
        Image.new("RGB", (1, 1), GROUND).save(d / "highlight_c.png")
    screen.save(config / "includes.binary/isolinux/splash.png", optimize=True)
    graphics = config / "includes.installer/usr/share/graphics"
    graphics.mkdir(parents=True, exist_ok=True)
    name_on_ground(font, (800, 75), 32, (16, 20)).save(graphics / "logo_debian.png", optimize=True)


if __name__ == "__main__":
    main()
