/**
 * NithronSync Mobile Application
 * Cross-platform file sync client for iOS and Android
 */

import React, {useEffect} from 'react';
import {StatusBar, useColorScheme} from 'react-native';
import {NavigationContainer, DefaultTheme, DarkTheme} from '@react-navigation/native';
import {SafeAreaProvider} from 'react-native-safe-area-context';
import {GestureHandlerRootView} from 'react-native-gesture-handler';

import {RootNavigator} from './src/navigation/RootNavigator';
import {useAuthStore} from './src/stores/authStore';
import {useSyncStore} from './src/stores/syncStore';

// Custom themes matching NithronOS branding
const NithronLightTheme = {
  ...DefaultTheme,
  colors: {
    ...DefaultTheme.colors,
    primary: '#2D7FF9',
    background: '#F8FAFC',
    card: '#FFFFFF',
    text: '#0F172A',
    border: '#E2E8F0',
    notification: '#A4F932',
  },
};

const NithronDarkTheme = {
  ...DarkTheme,
  colors: {
    ...DarkTheme.colors,
    primary: '#2D7FF9',
    background: '#0F172A',
    card: '#1E293B',
    text: '#F8FAFC',
    border: '#334155',
    notification: '#A4F932',
  },
};

export default function App(): React.JSX.Element {
  const colorScheme = useColorScheme();
  const {initialize: initAuth, isAuthenticated} = useAuthStore();
  const {initialize: initSync} = useSyncStore();

  useEffect(() => {
    // Initialize stores on app start
    initAuth();
  }, [initAuth]);

  useEffect(() => {
    // Initialize sync engine when authenticated
    if (isAuthenticated) {
      initSync();
    }
  }, [isAuthenticated, initSync]);

  return (
    <GestureHandlerRootView style={{flex: 1}}>
      <SafeAreaProvider>
        <NavigationContainer
          theme={colorScheme === 'dark' ? NithronDarkTheme : NithronLightTheme}>
          <StatusBar
            barStyle={colorScheme === 'dark' ? 'light-content' : 'dark-content'}
            backgroundColor={
              colorScheme === 'dark'
                ? NithronDarkTheme.colors.background
                : NithronLightTheme.colors.background
            }
          />
          <RootNavigator />
        </NavigationContainer>
      </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}

