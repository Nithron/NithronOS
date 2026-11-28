# NithronSync Mobile

The NithronSync app for Android (React Native 0.76). iOS comes later: the
JavaScript is shared, but there is no iOS project yet.

What it does today:

- **Pairs** with nOS from the pairing link the web interface shows
  (**Settings → Sync → Devices → Add Device**), by scanning its QR code or
  pasting the link.
- **Trusts nOS's self-signed certificate** by the server key in the link
  (see [Pinning](#pinning)).
- **Browses shares**: folders and files over WebDAV, a file's details,
  deleting a file on a share you may change.
- Shows this device's **activity** and **conflicts** as nOS records them,
  renames the device, and **unpairs** (removes the device in nOS).

It doesn't sync files to the phone yet. User guide:
[docs/sync/client-setup.md](../../docs/sync/client-setup.md#android-app).

## Build

Needs Node 20+, JDK 17 and the Android SDK (`ANDROID_HOME`; platform 35,
build tools 35, NDK 26.1.10909125, CMake 3.22.1).

```bash
npm ci
npm run typecheck && npm run lint && npm test
cd android && ./gradlew assembleDebug testDebugUnitTest
# app/build/outputs/apk/debug/app-debug.apk
```

The debug APK bundles its JavaScript, so it runs without Metro; with
`npm start` running it loads from Metro instead. `-PreactNativeArchitectures=arm64-v8a,x86_64`
builds fewer ABIs (what CI does). A release build is signed with the key in
`android/keystore.properties` (`storeFile`, `storePassword`, `keyAlias`,
`keyPassword`) and is unsigned without it.

CI (`Mobile (Android)` in `.github/workflows/ci.yml`) runs all of the above
and keeps the debug APK as an artifact for 14 days.

## Layout

```
src/api/          pairing link parser, nosd client (refresh once on 401), WebDAV listing, secure token storage
src/native/       the JavaScript side of the pinning module
src/stores/       zustand stores: the pairing, shares/activity/conflicts
src/screens/      auth (welcome, QR scanner, paste link), files, main tabs, settings
src/theme/        the Nithron tokens (a copy of @nithron/tokens 0.4.0) and the app's one mapping onto them
src/components/   icons by meaning, the pixel layer (marks, meters, loading), buttons, fields, the dialog, the toast
src/vendor/       the shared pixel icons (@nithron/icons 0.2.0), copied whole
assets/fonts/     Mona Sans, Monaspace Neon and Nithron Pixel, with their licences (linked by react-native.config.js)
android/app/src/main/java/com/nithron/sync/pinning/   key pinning for React Native's HTTP client
__tests__/        Jest tests (pairing link, WebDAV, client, theme, components, identity); android/app/src/test: JVM tests for pinning
```

## Look

The app follows the Nithron identity as the nOS web interface does
(nithron-site DESIGN-APPS.md): one ground with hairlines, light or dark as
the system is, no hue but the states (green for a pass, red with its cross
for a failure, gold with an open square for a warning), pixel icons on
whole device pixels. Every colour comes from `src/theme/nithron-tokens.js`
through `src/theme/theme.ts`; `__tests__/identity.test.ts` fails on a hex
value, a hue's name, a spinner, the system's alert or a shadow anywhere
else.

## Pinning

nOS's web certificate is self-signed, so no certificate authority vouches
for it. The pairing link's `key` is the SHA-256 of the server certificate's
public key (SubjectPublicKeyInfo), as the desktop client uses it
(`clients/sync-core/api/tls.go`). On Android, `MainApplication` installs an
OkHttp factory for React Native's networking whose trust manager accepts a
pinned host's connection only if the leaf certificate's key matches the pin
(no CA check for that host, which can't pass) and validates every other host
the normal way. A pinned host's name isn't checked against the certificate
(its key identifies it), and every request checks the pin again, so a pooled
connection from before a re-pairing isn't reused. Pins are kept in
SharedPreferences by the `NosPinning` native module, so they apply from the
first request after a restart.

Tokens are kept with react-native-keychain (Android Keystore-backed).
