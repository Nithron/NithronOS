/**
 * Device Settings Screen
 * Manage device information and settings
 */

import React, {useEffect, useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TextInput,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {format} from 'date-fns';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {Device} from '../../api/types';
import {formatBytes} from '../../utils/format';

const MAX_NAME_LENGTH = 64; // nosd's limit

export function DeviceSettingsScreen(): React.JSX.Element {
  const {colors} = useTheme();
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
    try {
      await renameDevice(trimmed);
    } catch (err) {
      Alert.alert("Couldn't rename the device", errorMessage(err));
    } finally {
      setIsSaving(false);
    }
  };

  if (!currentDevice) {
    return (
      <View
        style={[styles.centerContainer, {backgroundColor: colors.background}]}>
        {error ? (
          <Text style={{color: colors.text}}>{error}</Text>
        ) : (
          <ActivityIndicator size="large" color={colors.primary} />
        )}
      </View>
    );
  }

  return (
    <ScrollView
      style={[styles.container, {backgroundColor: colors.background}]}>
      {/* Device Icon */}
      <View style={styles.header}>
        <View
          style={[styles.deviceIcon, {backgroundColor: `${colors.primary}15`}]}>
          <Icon
            name={currentDevice.device_type === 'ios' ? 'apple' : 'android'}
            size={48}
            color={colors.primary}
          />
        </View>
      </View>

      {/* Device Name */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>
          DEVICE NAME
        </Text>
        <View
          style={[
            styles.inputCard,
            {backgroundColor: colors.card, borderColor: colors.border},
          ]}>
          <TextInput
            style={[styles.input, {color: colors.text}]}
            value={deviceName}
            onChangeText={setDeviceName}
            maxLength={MAX_NAME_LENGTH}
            accessibilityLabel="Device name"
            placeholder="Enter device name"
            placeholderTextColor={`${colors.text}50`}
          />
        </View>
        {hasChanges && (
          <TouchableOpacity
            accessibilityRole="button"
            style={[styles.saveButton, {backgroundColor: colors.primary}]}
            onPress={handleSave}
            disabled={isSaving}>
            {isSaving ? (
              <ActivityIndicator color="#FFFFFF" size="small" />
            ) : (
              <Text style={styles.saveButtonText}>Save Changes</Text>
            )}
          </TouchableOpacity>
        )}
      </View>

      {/* Device Info */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>
          DEVICE INFO
        </Text>
        <View
          style={[
            styles.infoCard,
            {backgroundColor: colors.card, borderColor: colors.border},
          ]}>
          <InfoRow label="Device ID" value={currentDevice.id} colors={colors} />
          <Divider color={colors.border} />
          <InfoRow
            label="Type"
            value={capitalize(currentDevice.device_type)}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Registered"
            value={format(new Date(currentDevice.created_at), 'MMM d, yyyy')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Last Sync"
            value={formatTime(currentDevice.last_sync_at)}
            colors={colors}
          />
        </View>
      </View>

      {/* Sync Stats */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>
          SYNC STATISTICS
        </Text>
        <View
          style={[
            styles.infoCard,
            {backgroundColor: colors.card, borderColor: colors.border},
          ]}>
          <InfoRow
            label="Total Synced"
            value={formatBytes(currentDevice.bytes_synced)}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Last Seen"
            value={formatTime(currentDevice.last_seen_at)}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Access"
            value={currentDevice.is_revoked ? 'Revoked' : 'Active'}
            valueColor={currentDevice.is_revoked ? '#EF4444' : '#10B981'}
            colors={colors}
          />
        </View>
      </View>
    </ScrollView>
  );
}

function capitalize(type: Device['device_type']): string {
  return type.charAt(0).toUpperCase() + type.slice(1);
}

function formatTime(time: string | undefined): string {
  return time ? format(new Date(time), 'MMM d, yyyy h:mm a') : 'Never';
}

interface InfoRowProps {
  label: string;
  value: string;
  valueColor?: string;
  colors: {text: string};
}

function InfoRow({label, value, valueColor, colors}: InfoRowProps) {
  return (
    <View style={styles.infoRow}>
      <Text style={[styles.infoLabel, {color: `${colors.text}70`}]}>
        {label}
      </Text>
      <Text
        style={[styles.infoValue, {color: valueColor || colors.text}]}
        numberOfLines={1}>
        {value}
      </Text>
    </View>
  );
}

function Divider({color}: {color: string}) {
  return <View style={[styles.divider, {backgroundColor: color}]} />;
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  centerContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  header: {
    alignItems: 'center',
    paddingVertical: 32,
  },
  deviceIcon: {
    width: 96,
    height: 96,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
  },
  section: {
    paddingHorizontal: 16,
    marginBottom: 24,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 8,
    marginLeft: 4,
  },
  inputCard: {
    borderRadius: 12,
    borderWidth: 1,
    padding: 4,
  },
  input: {
    fontSize: 16,
    padding: 12,
  },
  saveButton: {
    marginTop: 12,
    paddingVertical: 14,
    borderRadius: 10,
    alignItems: 'center',
  },
  saveButtonText: {
    color: '#FFFFFF',
    fontSize: 16,
    fontWeight: '600',
  },
  infoCard: {
    borderRadius: 12,
    borderWidth: 1,
    overflow: 'hidden',
  },
  infoRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 14,
  },
  infoLabel: {
    fontSize: 15,
  },
  infoValue: {
    fontSize: 15,
    fontWeight: '500',
    maxWidth: '60%',
  },
  divider: {
    height: 1,
    marginLeft: 14,
  },
});

export default DeviceSettingsScreen;
