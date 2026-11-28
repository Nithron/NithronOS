/**
 * Root Navigator
 * Main navigation structure for the app
 */

import React from 'react';
import {StyleSheet, View} from 'react-native';
import {
  createNativeStackNavigator,
  type NativeStackNavigationOptions,
} from '@react-navigation/native-stack';
import {createBottomTabNavigator} from '@react-navigation/bottom-tabs';

import {useAuthStore} from '../stores/authStore';
import {unresolvedCount, useSyncStore} from '../stores/syncStore';
import {HistoryIcon, SettingsIcon, ShareIcon} from '../components/icons';
import {makeTabBar, StackHeader} from '../components/navigation';
import {Working} from '../components/pixel';
import {metrics, usePalette} from '../theme/theme';
import {fileName} from '../utils/format';

// Auth Screens
import {WelcomeScreen} from '../screens/auth/WelcomeScreen';
import {QRScannerScreen} from '../screens/auth/QRScannerScreen';
import {ManualSetupScreen} from '../screens/auth/ManualSetupScreen';

// Main Screens
import {ActivityScreen} from '../screens/main/ActivityScreen';
import {ConflictsScreen} from '../screens/main/ConflictsScreen';
import {SettingsScreen} from '../screens/main/SettingsScreen';

// Detail Screens
import {FileDetailScreen} from '../screens/files/FileDetailScreen';
import {FolderScreen} from '../screens/files/FolderScreen';
import {SharesScreen} from '../screens/files/SharesScreen';
import {DeviceSettingsScreen} from '../screens/settings/DeviceSettingsScreen';
import {AboutScreen} from '../screens/settings/AboutScreen';

// Navigation types
export type AuthStackParamList = {
  Welcome: undefined;
  QRScanner: undefined;
  ManualSetup: undefined;
};

export type MainTabsParamList = {
  Files: undefined;
  Activity: undefined;
  Conflicts: undefined;
  Settings: undefined;
};

export type FilesStackParamList = {
  Shares: undefined;
  Folder: {shareId: string; path: string; name: string};
  FileDetail: {shareId: string; path: string};
};

export type SettingsStackParamList = {
  SettingsMain: undefined;
  DeviceSettings: undefined;
  About: undefined;
};

export type RootStackParamList = {
  Auth: undefined;
  Main: undefined;
};

// Create navigators
const RootStack = createNativeStackNavigator<RootStackParamList>();
const AuthStack = createNativeStackNavigator<AuthStackParamList>();
const MainTabs = createBottomTabNavigator<MainTabsParamList>();
const FilesStack = createNativeStackNavigator<FilesStackParamList>();
const SettingsStack = createNativeStackNavigator<SettingsStackParamList>();

/** Every stack: the app's own header, and screens on the ground. */
function useStackOptions(): NativeStackNavigationOptions {
  const p = usePalette();
  return {
    header: StackHeader,
    contentStyle: {backgroundColor: p.page},
  };
}

// Auth Navigator
function AuthNavigator() {
  return (
    <AuthStack.Navigator screenOptions={useStackOptions()}>
      <AuthStack.Screen
        name="Welcome"
        component={WelcomeScreen}
        options={{headerShown: false}}
      />
      <AuthStack.Screen
        name="QRScanner"
        component={QRScannerScreen}
        options={{title: 'Scan the QR code'}}
      />
      <AuthStack.Screen
        name="ManualSetup"
        component={ManualSetupScreen}
        options={{title: 'Pairing link'}}
      />
    </AuthStack.Navigator>
  );
}

// Files Navigator
function FilesNavigator() {
  return (
    <FilesStack.Navigator screenOptions={useStackOptions()}>
      <FilesStack.Screen
        name="Shares"
        component={SharesScreen}
        options={{title: 'Shares'}}
      />
      <FilesStack.Screen
        name="Folder"
        component={FolderScreen}
        options={({route}) => ({title: route.params.name})}
      />
      <FilesStack.Screen
        name="FileDetail"
        component={FileDetailScreen}
        options={({route}) => ({title: fileName(route.params.path)})}
      />
    </FilesStack.Navigator>
  );
}

// Settings Navigator
function SettingsNavigator() {
  return (
    <SettingsStack.Navigator screenOptions={useStackOptions()}>
      <SettingsStack.Screen
        name="SettingsMain"
        component={SettingsScreen}
        options={{title: 'Settings'}}
      />
      <SettingsStack.Screen
        name="DeviceSettings"
        component={DeviceSettingsScreen}
        options={{title: 'This device'}}
      />
      <SettingsStack.Screen
        name="About"
        component={AboutScreen}
        options={{title: 'About'}}
      />
    </SettingsStack.Navigator>
  );
}

// Words with their icons beside them; Conflicts has no drawing yet, so it
// stands as its word.
const TabBar = makeTabBar({
  Files: ShareIcon,
  Activity: HistoryIcon,
  Settings: SettingsIcon,
});

// Main Tab Navigator
function MainNavigator() {
  const conflicts = useSyncStore(s => unresolvedCount(s.conflicts));

  return (
    <MainTabs.Navigator tabBar={TabBar} screenOptions={{headerShown: false}}>
      <MainTabs.Screen
        name="Files"
        component={FilesNavigator}
        options={{tabBarLabel: 'Files'}}
      />
      <MainTabs.Screen
        name="Activity"
        component={ActivityScreen}
        options={{tabBarLabel: 'Activity'}}
      />
      <MainTabs.Screen
        name="Conflicts"
        component={ConflictsScreen}
        options={{
          tabBarLabel: 'Conflicts',
          tabBarBadge: conflicts > 0 ? conflicts : undefined,
        }}
      />
      <MainTabs.Screen
        name="Settings"
        component={SettingsNavigator}
        options={{tabBarLabel: 'Settings'}}
      />
    </MainTabs.Navigator>
  );
}

/** While the pairing is read from the secure store. */
function Starting(): React.JSX.Element {
  const p = usePalette();
  return (
    <View style={[startingStyles.screen, {backgroundColor: p.page}]}>
      <Working size={24}>Starting</Working>
    </View>
  );
}

const startingStyles = StyleSheet.create({
  screen: {
    flex: 1,
    justifyContent: 'center',
    paddingHorizontal: metrics.gutter,
  },
});

// Root Navigator
export function RootNavigator() {
  const status = useAuthStore(s => s.status);

  if (status === 'starting') {
    return <Starting />;
  }

  return (
    <RootStack.Navigator screenOptions={{headerShown: false}}>
      {status === 'paired' ? (
        <RootStack.Screen name="Main" component={MainNavigator} />
      ) : (
        <RootStack.Screen name="Auth" component={AuthNavigator} />
      )}
    </RootStack.Navigator>
  );
}

export default RootNavigator;
