/**
 * Device Settings Screen
 * Manage device information and settings
 */

import React, {useState} from 'react';
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

import {useAuthStore} from '../../stores/authStore';
import {apiClient} from '../../api/client';

export function DeviceSettingsScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const {currentDevice, refreshDeviceInfo} = useAuthStore();
  
  const [deviceName, setDeviceName] = useState(currentDevice?.name || '');
  const [isSaving, setIsSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);

  const handleNameChange = (text: string) => {
    setDeviceName(text);
    setHasChanges(text !== currentDevice?.name);
  };

  const handleSave = async () => {
    if (!hasChanges || !currentDevice) return;
    
    setIsSaving(true);
    try {
      await apiClient.updateDevice(currentDevice.id, deviceName);
      await refreshDeviceInfo();
      setHasChanges(false);
      Alert.alert('Success', 'Device name updated');
    } catch (error) {
      Alert.alert('Error', 'Failed to update device name');
    } finally {
      setIsSaving(false);
    }
  };

  if (!currentDevice) {
    return (
      <View style={[styles.centerContainer, {backgroundColor: colors.background}]}>
        <ActivityIndicator size="large" color={colors.primary} />
      </View>
    );
  }

  return (
    <ScrollView style={[styles.container, {backgroundColor: colors.background}]}>
      {/* Device Icon */}
      <View style={styles.header}>
        <View style={[styles.deviceIcon, {backgroundColor: `${colors.primary}15`}]}>
          <Icon
            name={currentDevice.platform === 'ios' ? 'apple' : 'android'}
            size={48}
            color={colors.primary}
          />
        </View>
      </View>

      {/* Device Name */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>DEVICE NAME</Text>
        <View style={[styles.inputCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <TextInput
            style={[styles.input, {color: colors.text}]}
            value={deviceName}
            onChangeText={handleNameChange}
            placeholder="Enter device name"
            placeholderTextColor={`${colors.text}50`}
          />
        </View>
        {hasChanges && (
          <TouchableOpacity
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
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>DEVICE INFO</Text>
        <View style={[styles.infoCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <InfoRow
            label="Device ID"
            value={currentDevice.id}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Platform"
            value={currentDevice.platform.charAt(0).toUpperCase() + currentDevice.platform.slice(1)}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Type"
            value={currentDevice.type.charAt(0).toUpperCase() + currentDevice.type.slice(1)}
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
            value={currentDevice.last_sync 
              ? format(new Date(currentDevice.last_sync), 'MMM d, yyyy h:mm a')
              : 'Never'}
            colors={colors}
          />
        </View>
      </View>

      {/* Sync Stats */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>SYNC STATISTICS</Text>
        <View style={[styles.infoCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <InfoRow
            label="Total Synced"
            value={formatBytes(currentDevice.sync_bytes || 0)}
            colors={colors}
          />
          <Divider color={colors.border} />
          <InfoRow
            label="Status"
            value={currentDevice.is_online ? 'Online' : 'Offline'}
            valueColor={currentDevice.is_online ? '#10B981' : '#EF4444'}
            colors={colors}
          />
        </View>
      </View>
    </ScrollView>
  );
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
      <Text style={[styles.infoLabel, {color: `${colors.text}70`}]}>{label}</Text>
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

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
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

