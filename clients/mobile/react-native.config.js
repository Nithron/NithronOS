/**
 * The type faces of the Nithron identity, bundled with the app (a NAS may
 * be offline; nothing is loaded from another server). `npx
 * react-native-asset` links them: it copies assets/fonts into
 * android/app/src/main/assets/fonts and lists them under UIAppFonts in
 * ios/NithronSync/Info.plist. Both copies are committed; a test checks they
 * are the same files.
 */
module.exports = {
  assets: ['./assets/fonts/'],
};
