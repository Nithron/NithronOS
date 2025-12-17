# NithronSync Mobile

Cross-platform mobile client for NithronOS file synchronization, supporting iOS and Android.

## Features

- **QR Code Setup** — Scan a QR code from your NithronOS dashboard for instant configuration
- **File Browser** — Browse and manage your synced files on the go
- **Background Sync** — Automatic file synchronization in the background
- **Conflict Resolution** — Visual interface for resolving file conflicts
- **Activity History** — Track all sync operations
- **Offline Support** — Access cached files when offline

## Requirements

### Development

- Node.js 20+
- React Native CLI
- Xcode 15+ (for iOS development)
- Android Studio / Android SDK (for Android development)

### Runtime

- iOS 15.0+
- Android 10+ (API 29+)

## Installation

### Development Setup

```bash
# Install dependencies
npm install

# Install iOS pods (macOS only)
cd ios && pod install && cd ..

# Start Metro bundler
npm start
```

### Running on iOS

```bash
# Run on iOS simulator
npm run ios

# Run on connected device
npm run ios -- --device "iPhone Name"
```

### Running on Android

```bash
# Run on Android emulator/device
npm run android
```

## Project Structure

```
src/
├── api/                 # API client and types
│   ├── client.ts        # NithronSync API client
│   └── types.ts         # TypeScript type definitions
├── navigation/          # React Navigation setup
│   └── RootNavigator.tsx
├── screens/             # Screen components
│   ├── auth/            # Authentication screens
│   │   ├── WelcomeScreen.tsx
│   │   ├── QRScannerScreen.tsx
│   │   └── ManualSetupScreen.tsx
│   ├── main/            # Main tab screens
│   │   ├── FilesScreen.tsx
│   │   ├── ActivityScreen.tsx
│   │   ├── ConflictsScreen.tsx
│   │   └── SettingsScreen.tsx
│   ├── files/           # File management screens
│   │   ├── SharesScreen.tsx
│   │   ├── FolderScreen.tsx
│   │   └── FileDetailScreen.tsx
│   ├── conflicts/       # Conflict resolution screens
│   │   └── ConflictDetailScreen.tsx
│   └── settings/        # Settings screens
│       ├── DeviceSettingsScreen.tsx
│       ├── SyncSettingsScreen.tsx
│       └── AboutScreen.tsx
├── stores/              # Zustand state management
│   ├── authStore.ts     # Authentication state
│   └── syncStore.ts     # Sync state
└── components/          # Reusable components
```

## Authentication Flow

1. **Welcome Screen** — User chooses QR code scan or manual setup
2. **QR Scanner** — Scan QR code from NithronOS dashboard
3. **Manual Setup** — Enter server URL and register device
4. **Main App** — Access files, activity, and settings

## Configuration

The app stores configuration in AsyncStorage:

| Key | Description |
|-----|-------------|
| `@nithron_server_url` | Server URL |
| `@nithron_device_id` | Device ID |
| `@nithron_device_token` | Access token |
| `@nithron_refresh_token` | Refresh token |
| `@nithron_token_expires` | Token expiration |

## Building for Production

### iOS

```bash
# Build release archive
cd ios
xcodebuild -workspace NithronSync.xcworkspace -scheme NithronSync -configuration Release archive -archivePath build/NithronSync.xcarchive

# Export IPA
xcodebuild -exportArchive -archivePath build/NithronSync.xcarchive -exportPath build -exportOptionsPlist ExportOptions.plist
```

### Android

```bash
# Build release APK
cd android
./gradlew assembleRelease

# Build release AAB (for Play Store)
./gradlew bundleRelease
```

## Testing

```bash
# Run tests
npm test

# Run with coverage
npm test -- --coverage

# Lint code
npm run lint

# Type check
npm run typecheck
```

## Contributing

See the main [CONTRIBUTING.md](../../CONTRIBUTING.md) for guidelines.

## License

NithronSync Mobile is part of NithronOS and is licensed under the NithronOS Community License (NCL).

