/**
 * NithronSync Mobile Application
 * Browses the shares of a paired nOS machine
 */

import React, {useEffect} from 'react';
import {StatusBar, useColorScheme} from 'react-native';
import {
  NavigationContainer,
  DefaultTheme,
  DarkTheme,
} from '@react-navigation/native';
import {SafeAreaProvider} from 'react-native-safe-area-context';

import {RootNavigator} from './src/navigation/RootNavigator';
import {useAuthStore} from './src/stores/authStore';
import {useSyncStore} from './src/stores/syncStore';

// Custom themes matching nOS branding
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
  const initialize = useAuthStore(s => s.initialize);
  const isPaired = useAuthStore(s => s.status === 'paired');
  const {refreshShares, refreshConflicts, reset} = useSyncStore();

  useEffect(() => {
    initialize();
  }, [initialize]);

  useEffect(() => {
    if (isPaired) {
      refreshShares();
      refreshConflicts();
    } else {
      reset();
    }
  }, [isPaired, refreshShares, refreshConflicts, reset]);

  const theme = colorScheme === 'dark' ? NithronDarkTheme : NithronLightTheme;

  return (
    <SafeAreaProvider>
      <NavigationContainer theme={theme}>
        <StatusBar
          barStyle={colorScheme === 'dark' ? 'light-content' : 'dark-content'}
          backgroundColor={theme.colors.background}
        />
        <RootNavigator />
      </NavigationContainer>
    </SafeAreaProvider>
  );
}
