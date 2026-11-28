/**
 * Settings: the server this phone is paired with, this device, and
 * unpairing (confirmed in the app's own dialog).
 */

import React, {useState} from 'react';
import {ScrollView, Text, View} from 'react-native';
import {useNavigation} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {useSyncStore} from '../../stores/syncStore';
import {SettingsStackParamList} from '../../navigation/RootNavigator';
import {Button} from '../../components/Button';
import {ConfirmDialog} from '../../components/ConfirmDialog';
import {RefreshIcon, SignOutIcon} from '../../components/icons';
import {Group, Row} from '../../components/layout';
import {StatusMark} from '../../components/pixel';
import {showToast} from '../../components/Toast';
import {APP_VERSION} from '../../version';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';

type SettingsNavigationProp = NativeStackNavigationProp<
  SettingsStackParamList,
  'SettingsMain'
>;

export function SettingsScreen(): React.JSX.Element {
  const styles = useThemedStyles(settingsStyles);
  const navigation = useNavigation<SettingsNavigationProp>();
  const {device, serverUrl, error, isBusy, unpair, refreshDevice} =
    useAuthStore();
  const resetData = useSyncStore(s => s.reset);
  const [confirming, setConfirming] = useState(false);
  const [unpairError, setUnpairError] = useState<string | null>(null);

  const doUnpair = async () => {
    setUnpairError(null);
    try {
      const {revoked} = await unpair();
      setConfirming(false);
      resetData();
      if (!revoked) {
        showToast(
          'warning',
          "Unpaired on this phone. nOS couldn't be told, so it still lists the device: remove it under Sync → Devices.",
        );
      }
    } catch (err) {
      setUnpairError(`Couldn't unpair. ${errorMessage(err)}`);
    }
  };

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      <Group label="Server">
        <View style={styles.server}>
          <StatusMark state={error ? 'failed' : 'ok'}>
            {error ? "Can't reach the server" : 'Paired'}
          </StatusMark>
          {error ? <Text style={styles.reason}>{error}</Text> : null}
          {serverUrl ? (
            <Text style={styles.url} selectable>
              {serverUrl}
            </Text>
          ) : null}
          {error ? (
            <Button
              icon={RefreshIcon}
              label="Try again"
              onPress={refreshDevice}
              compact
            />
          ) : null}
        </View>
      </Group>

      <Group label="Device">
        <Row
          title={device?.device_name ?? 'This device'}
          detail="Name, sync statistics"
          onPress={() => navigation.navigate('DeviceSettings')}
        />
        <Row
          title="About"
          detail="Version, links"
          onPress={() => navigation.navigate('About')}
        />
      </Group>

      <View style={styles.unpair}>
        <Button
          icon={SignOutIcon}
          label={isBusy ? 'Unpairing' : 'Unpair this phone'}
          onPress={() => {
            setUnpairError(null);
            setConfirming(true);
          }}
          disabled={isBusy}
        />
        {unpairError && !confirming ? (
          <StatusMark state="failed" textStyle={type.fine}>
            {unpairError}
          </StatusMark>
        ) : null}
      </View>

      <Text style={styles.footer}>NithronSync {APP_VERSION}</Text>

      <ConfirmDialog
        visible={confirming}
        title="Unpair this phone?"
        message="nOS removes the device and this phone forgets the pairing. To use it again, pair it with a new QR code."
        actions={[{label: 'Unpair', onPress: doUnpair}]}
        onCancel={() => setConfirming(false)}
        error={unpairError}
        busy={isBusy}
      />
    </ScrollView>
  );
}

const settingsStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {paddingBottom: metrics.space6},
  server: {
    gap: metrics.space2,
    paddingHorizontal: metrics.gutter,
    paddingVertical: metrics.space3,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
    alignItems: 'flex-start' as const,
  },
  reason: {...type.fine, color: p.ink2},
  url: {...type.mono, color: p.ink},
  unpair: {
    paddingHorizontal: metrics.gutter,
    marginTop: metrics.space6,
    gap: metrics.space3,
  },
  footer: {
    ...type.label,
    color: p.ink2,
    paddingHorizontal: metrics.gutter,
    marginTop: metrics.space6,
  },
});

export default SettingsScreen;
