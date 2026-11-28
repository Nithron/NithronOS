/**
 * NithronSync Mobile Application
 * Browses the shares of a paired nOS machine
 */

import React, {useEffect, useMemo} from 'react';
import {StatusBar, StyleSheet, View} from 'react-native';
import {NavigationContainer} from '@react-navigation/native';
import {SafeAreaProvider} from 'react-native-safe-area-context';

import {RootNavigator} from './src/navigation/RootNavigator';
import {ToastHost} from './src/components/Toast';
import {useAuthStore} from './src/stores/authStore';
import {useSyncStore} from './src/stores/syncStore';
import {navigationTheme, usePalette} from './src/theme/theme';

export default function App(): React.JSX.Element {
  // The Nithron tokens, light or dark as the system is.
  const p = usePalette();
  const theme = useMemo(() => navigationTheme(p), [p]);
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

  return (
    <SafeAreaProvider>
      <View style={[styles.app, {backgroundColor: p.page}]}>
        <NavigationContainer theme={theme}>
          <StatusBar
            barStyle={p.scheme === 'dark' ? 'light-content' : 'dark-content'}
            backgroundColor={p.page}
          />
          <RootNavigator />
        </NavigationContainer>
        <ToastHost />
      </View>
    </SafeAreaProvider>
  );
}

const styles = StyleSheet.create({
  app: {flex: 1},
});
