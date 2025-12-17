/**
 * Root Navigator
 * Main navigation structure for the app
 */

import React from 'react';
import {createNativeStackNavigator} from '@react-navigation/native-stack';
import {createBottomTabNavigator} from '@react-navigation/bottom-tabs';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {useAuthStore} from '../stores/authStore';

// Auth Screens
import {WelcomeScreen} from '../screens/auth/WelcomeScreen';
import {QRScannerScreen} from '../screens/auth/QRScannerScreen';
import {ManualSetupScreen} from '../screens/auth/ManualSetupScreen';

// Main Screens
import {FilesScreen} from '../screens/main/FilesScreen';
import {ActivityScreen} from '../screens/main/ActivityScreen';
import {ConflictsScreen} from '../screens/main/ConflictsScreen';
import {SettingsScreen} from '../screens/main/SettingsScreen';

// Detail Screens
import {FileDetailScreen} from '../screens/files/FileDetailScreen';
import {FolderScreen} from '../screens/files/FolderScreen';
import {SharesScreen} from '../screens/files/SharesScreen';
import {ConflictDetailScreen} from '../screens/conflicts/ConflictDetailScreen';
import {DeviceSettingsScreen} from '../screens/settings/DeviceSettingsScreen';
import {SyncSettingsScreen} from '../screens/settings/SyncSettingsScreen';
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
  SyncSettings: undefined;
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

// Auth Navigator
function AuthNavigator() {
  const {colors} = useTheme();

  return (
    <AuthStack.Navigator
      screenOptions={{
        headerStyle: {backgroundColor: colors.card},
        headerTintColor: colors.text,
        headerTitleStyle: {fontWeight: '600'},
      }}>
      <AuthStack.Screen
        name="Welcome"
        component={WelcomeScreen}
        options={{headerShown: false}}
      />
      <AuthStack.Screen
        name="QRScanner"
        component={QRScannerScreen}
        options={{title: 'Scan QR Code'}}
      />
      <AuthStack.Screen
        name="ManualSetup"
        component={ManualSetupScreen}
        options={{title: 'Manual Setup'}}
      />
    </AuthStack.Navigator>
  );
}

// Files Navigator
function FilesNavigator() {
  const {colors} = useTheme();

  return (
    <FilesStack.Navigator
      screenOptions={{
        headerStyle: {backgroundColor: colors.card},
        headerTintColor: colors.text,
        headerTitleStyle: {fontWeight: '600'},
      }}>
      <FilesStack.Screen
        name="Shares"
        component={SharesScreen}
        options={{title: 'My Shares'}}
      />
      <FilesStack.Screen
        name="Folder"
        component={FolderScreen}
        options={({route}) => ({title: route.params.name})}
      />
      <FilesStack.Screen
        name="FileDetail"
        component={FileDetailScreen}
        options={{title: 'File Details'}}
      />
    </FilesStack.Navigator>
  );
}

// Settings Navigator
function SettingsNavigator() {
  const {colors} = useTheme();

  return (
    <SettingsStack.Navigator
      screenOptions={{
        headerStyle: {backgroundColor: colors.card},
        headerTintColor: colors.text,
        headerTitleStyle: {fontWeight: '600'},
      }}>
      <SettingsStack.Screen
        name="SettingsMain"
        component={SettingsScreen}
        options={{title: 'Settings'}}
      />
      <SettingsStack.Screen
        name="DeviceSettings"
        component={DeviceSettingsScreen}
        options={{title: 'Device'}}
      />
      <SettingsStack.Screen
        name="SyncSettings"
        component={SyncSettingsScreen}
        options={{title: 'Sync Settings'}}
      />
      <SettingsStack.Screen
        name="About"
        component={AboutScreen}
        options={{title: 'About'}}
      />
    </SettingsStack.Navigator>
  );
}

// Main Tab Navigator
function MainNavigator() {
  const {colors} = useTheme();

  return (
    <MainTabs.Navigator
      screenOptions={{
        headerShown: false,
        tabBarStyle: {
          backgroundColor: colors.card,
          borderTopColor: colors.border,
        },
        tabBarActiveTintColor: colors.primary,
        tabBarInactiveTintColor: colors.text,
      }}>
      <MainTabs.Screen
        name="Files"
        component={FilesNavigator}
        options={{
          tabBarLabel: 'Files',
          tabBarIcon: ({color, size}) => (
            <Icon name="folder-outline" size={size} color={color} />
          ),
        }}
      />
      <MainTabs.Screen
        name="Activity"
        component={ActivityScreen}
        options={{
          tabBarLabel: 'Activity',
          tabBarIcon: ({color, size}) => (
            <Icon name="history" size={size} color={color} />
          ),
        }}
      />
      <MainTabs.Screen
        name="Conflicts"
        component={ConflictsScreen}
        options={{
          tabBarLabel: 'Conflicts',
          tabBarIcon: ({color, size}) => (
            <Icon name="alert-circle-outline" size={size} color={color} />
          ),
          tabBarBadge: undefined, // Will be set dynamically
        }}
      />
      <MainTabs.Screen
        name="Settings"
        component={SettingsNavigator}
        options={{
          tabBarLabel: 'Settings',
          tabBarIcon: ({color, size}) => (
            <Icon name="cog-outline" size={size} color={color} />
          ),
        }}
      />
    </MainTabs.Navigator>
  );
}

// Root Navigator
export function RootNavigator() {
  const {isInitialized, isAuthenticated} = useAuthStore();

  if (!isInitialized) {
    // Could show a splash screen here
    return null;
  }

  return (
    <RootStack.Navigator screenOptions={{headerShown: false}}>
      {isAuthenticated ? (
        <RootStack.Screen name="Main" component={MainNavigator} />
      ) : (
        <RootStack.Screen name="Auth" component={AuthNavigator} />
      )}
    </RootStack.Navigator>
  );
}

export default RootNavigator;

