/**
 * Device Settings Screen
 * This device's name (the one first action saves it) and its record in nOS.
 */

import React, {useEffect, useState} from 'react';
import {ScrollView, View} from 'react-native';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {Device} from '../../api/types';
import {Button} from '../../components/Button';
import {Field} from '../../components/Field';
import {Group, InfoRow} from '../../components/layout';
import {StatusMark, Working} from '../../components/pixel';
import {showToast} from '../../components/Toast';
import {formatDate, formatDateTime} from '../../utils/datetime';
import {formatBytes} from '../../utils/format';
import {metrics, Palette, useThemedStyles} from '../../theme/theme';

const MAX_NAME_LENGTH = 64; // nosd's limit

const TYPE_NAMES: Record<Device['device_type'], string> = {
  windows: 'Windows',
  linux: 'Linux',
  macos: 'macOS',
  android: 'Android',
  ios: 'iOS',
  unknown: 'Unknown',
};

export function DeviceSettingsScreen(): React.JSX.Element {
  const styles = useThemedStyles(deviceStyles);
  const {
    device: currentDevice,
    error,
    refreshDevice,
    renameDevice,
  } = useAuthStore();

  const [deviceName, setDeviceName] = useState(
    currentDevice?.device_name ?? '',
  );
  const [isSaving, setIsSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (!currentDevice) {
      refreshDevice();
    }
  }, [currentDevice, refreshDevice]);

  useEffect(() => {
    setDeviceName(currentDevice?.device_name ?? '');
  }, [currentDevice?.device_name]);

  const trimmed = deviceName.trim();
  const hasChanges = trimmed !== '' && trimmed !== currentDevice?.device_name;

  const handleSave = async () => {
    if (!hasChanges) {
      return;
    }
    setIsSaving(true);
    setSaveError(null);
    try {
      await renameDevice(trimmed);
      showToast('ok', `Renamed this device to ${trimmed}.`);
    } catch (err) {
      setSaveError(`Couldn't rename the device. ${errorMessage(err)}`);
    } finally {
      setIsSaving(false);
    }
  };

  if (!currentDevice) {
    return (
      <View style={[styles.screen, styles.inset, styles.problem]}>
        {error ? (
          <>
            <StatusMark state="failed">{error}</StatusMark>
            <Button kind="primary" label="Try again" onPress={refreshDevice} />
          </>
        ) : (
          <Working>Loading this device</Working>
        )}
      </View>
    );
  }

  return (
    <ScrollView
      style={styles.screen}
      contentContainerStyle={styles.content}
      keyboardShouldPersistTaps="handled">
      <View style={[styles.inset, styles.form]}>
        <Field
          label="Device name"
          value={deviceName}
          onChangeText={text => {
            setDeviceName(text);
            setSaveError(null);
          }}
          maxLength={MAX_NAME_LENGTH}
          placeholder="Name of this device"
          error={saveError}
          returnKeyType="done"
          onSubmitEditing={handleSave}
        />
        {hasChanges ? (
          <Button
            kind="primary"
            label={isSaving ? 'Saving' : 'Save the name'}
            onPress={handleSave}
            disabled={isSaving}
          />
        ) : null}
      </View>

      <Group label="Device">
        <InfoRow label="Device ID" value={currentDevice.id} />
        <InfoRow label="Type" value={TYPE_NAMES[currentDevice.device_type]} />
        <InfoRow
          label="Registered"
          value={formatDate(currentDevice.created_at)}
        />
        <InfoRow
          label="Last sync"
          value={formatWhen(currentDevice.last_sync_at)}
        />
      </Group>

      <Group label="Sync statistics">
        <InfoRow
          label="Total synced"
          value={formatBytes(currentDevice.bytes_synced)}
        />
        <InfoRow
          label="Last seen"
          value={formatWhen(currentDevice.last_seen_at)}
        />
        <InfoRow label="Access">
          <StatusMark state={currentDevice.is_revoked ? 'failed' : 'ok'}>
            {currentDevice.is_revoked ? 'Revoked' : 'Active'}
          </StatusMark>
        </InfoRow>
      </Group>
    </ScrollView>
  );
}

function formatWhen(time: string | undefined): string {
  return time ? formatDateTime(time) : 'Never';
}

const deviceStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {paddingBottom: metrics.space6},
  inset: {padding: metrics.gutter},
  problem: {gap: metrics.space4, alignItems: 'flex-start' as const},
  form: {gap: metrics.space3},
});

export default DeviceSettingsScreen;
