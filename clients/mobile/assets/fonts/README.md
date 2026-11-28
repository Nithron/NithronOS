# Type faces

The three faces of the Nithron identity, bundled with NithronSync (nothing
is loaded from another server). React Native reads TrueType or OpenType,
not WOFF2, so each file is the upstream release file decompressed from
WOFF2 with fontTools (`python3 -m fontTools.ttLib.woff2 decompress`), its
outlines and names unchanged; only the file names were set to the
PostScript names, which is how both platforms find them. The same release
archives as the web interface's (web/public/fonts/README.md).

| File | Face | Source | SHA-256 of the source |
|------|------|--------|-----------------------|
| MonaSans-Regular.otf | Mona Sans, Regular (400) | github/mona-sans v2.0.27, mona-sans-webfonts-v2.0.27.zip: fonts/webfonts/static/MonaSans-Regular.woff2 | c87e8077d26694c2c41596475d2948cf3e697852db7e75e40a953e9baae7c02b |
| MonaSans-SemiBold.otf | Mona Sans, SemiBold (600) | the same archive: fonts/webfonts/static/MonaSans-SemiBold.woff2 | b55052d414ca8a800db737e423a0b896cc1002fbf63a29468ac90a135bccd134 |
| MonaspaceNeon-Regular.otf | Monaspace Neon, Regular | githubnext/monaspace v1.400, monaspace-webfont-static-v1.400.zip: Static Web Fonts/Monaspace Neon/MonaspaceNeon-Regular.woff2 (web/public/fonts) | 6b6980f24d115e98e96b1a5d06a5a7fe6454de9b216d479a7e6923c0456a5f41 |
| NithronPixel-Regular.ttf | Nithron Pixel | @nithron/tokens 0.4.0, fonts/nithron-pixel.ttf, unchanged | e34ab3f3e4abf86f20817c65c62e05522f8a7109316c3c77e3d35e4f15c14246 |

Mona Sans comes as two static weights rather than the variable file the
web interface uses: a variable face's default instance is ExtraLight
(200), and React Native picks no weight inside it. SemiBold (600) stands in
for the tokens' 560 (titles, buttons, the current item), the nearest weight
of the static set.

Licences: Mona Sans and Monaspace under the SIL Open Font License 1.1
(MONA-SANS-LICENSE.txt, MONASPACE-LICENSE.txt), Nithron Pixel under the
Nithron Community License v1.0 (NITHRON-PIXEL-LICENSE.txt). The fonts stay
under their licences whatever the app's own licence is, and are never sold
by themselves.

`react-native.config.js` names this folder; `npx react-native-asset` links
it (android/app/src/main/assets/fonts, UIAppFonts in the iOS Info.plist).
`__tests__/identity.test.ts` checks the Android copies match these files.
