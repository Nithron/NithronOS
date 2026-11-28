# Type faces

The two faces of the Nithron identity, served by nOS itself (a NAS may be
offline; nothing is loaded from another server). Both are the upstream
release files, unchanged; only the file names were simplified.

| File | Face | Source | SHA-256 |
|------|------|--------|---------|
| MonaSansVF-opsz-wght.woff2 | Mona Sans, variable (optical size, weight) | github/mona-sans v2.0.27, mona-sans-webfonts-v2.0.27.zip: fonts/webfonts/variable/MonaSansVF[opsz,wght].woff2 | 62e40f6e14e5bbb97132b4513a4d97319ab6aaa46996cf46c7a9f357edadb662 |
| MonaspaceNeon-Regular.woff2 | Monaspace Neon, regular | githubnext/monaspace v1.400, monaspace-webfont-static-v1.400.zip: Static Web Fonts/Monaspace Neon/MonaspaceNeon-Regular.woff2 | 6b6980f24d115e98e96b1a5d06a5a7fe6454de9b216d479a7e6923c0456a5f41 |

Licences: SIL Open Font License 1.1, in MONA-SANS-LICENSE.txt and
MONASPACE-LICENSE.txt (Monaspace's from the repository at tag v1.400). The
fonts stay under that licence whatever nOS's own licence is, and are never
sold by themselves. src/styles.css declares them under the family names
the tokens use ("Mona Sans Variable", "Monaspace Neon").
