/**
 * Settings Screen
 * Main settings menu
 */

import React from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Alert,
  Switch,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {useAuthStore} from '../../stores/authStore';
import {useSyncStore, SyncStatus} from '../../stores/syncStore';
import {SettingsStackParamList} from '../../navigation/RootNavigator';

type SettingsNavigationProp = NativeStackNavigationProp<SettingsStackParamList, 'SettingsMain'>;

export function SettingsScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation<SettingsNavigationProp>();
  const {currentDevice, serverInfo, logout} = useAuthStore();
  const {status, pauseSync, resumeSync, syncNow} = useSyncStore();

  const isPaused = status === 'paused';
  const isSyncing = status === 'syncing';

  const handleLogout = () => {
    Alert.alert(
      'Disconnect Device',
      'This will remove this device from sync. You can reconnect later by scanning a new QR code.',
      [
        {text: 'Cancel', style: 'cancel'},
        {
          text: 'Disconnect',
          style: 'destructive',
          onPress: logout,
        },
      ]
    );
  };

  const handleToggleSync = () => {
    if (isPaused) {
      resumeSync();
    } else {
      pauseSync();
    }
  };

  const handleSyncNow = () => {
    if (isSyncing) return;
    syncNow();
  };

  return (
    <ScrollView style={[styles.container, {backgroundColor: colors.background}]}>
      {/* Sync Status Card */}
      <View style={[styles.statusCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
        <View style={styles.statusHeader}>
          <View style={[styles.statusIcon, {backgroundColor: getStatusColor(status) + '20'}]}>
            <Icon name={getStatusIcon(status)} size={24} color={getStatusColor(status)} />
          </View>
          <View style={styles.statusInfo}>
            <Text style={[styles.statusTitle, {color: colors.text}]}>
              {getStatusText(status)}
            </Text>
            <Text style={[styles.statusSubtitle, {color: `${colors.text}60`}]}>
              {serverInfo?.url || 'Not connected'}
            </Text>
          </View>
        </View>
        
        <View style={styles.statusActions}>
          <TouchableOpacity
            style={[styles.syncButton, {backgroundColor: colors.primary}]}
            onPress={handleSyncNow}
            disabled={isSyncing}>
            <Icon
              name={isSyncing ? 'sync' : 'sync'}
              size={18}
              color="#FFFFFF"
            />
            <Text style={styles.syncButtonText}>
              {isSyncing ? 'Syncing...' : 'Sync Now'}
            </Text>
          </TouchableOpacity>
          
          <View style={styles.pauseToggle}>
            <Text style={[styles.pauseLabel, {color: colors.text}]}>
              {isPaused ? 'Paused' : 'Active'}
            </Text>
            <Switch
              value={!isPaused}
              onValueChange={handleToggleSync}
              trackColor={{false: colors.border, true: colors.primary}}
            />
          </View>
        </View>
      </View>

      {/* Device Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>DEVICE</Text>
        <View style={[styles.sectionCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <SettingsRow
            icon="cellphone"
            title={currentDevice?.name || 'This Device'}
            subtitle={`${currentDevice?.platform || 'Unknown'} • Last sync: Recently`}
            onPress={() => navigation.navigate('DeviceSettings')}
            colors={colors}
          />
        </View>
      </View>

      {/* Sync Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>SYNC</Text>
        <View style={[styles.sectionCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <SettingsRow
            icon="folder-sync"
            title="Sync Settings"
            subtitle="Bandwidth, WiFi-only, auto-upload"
            onPress={() => navigation.navigate('SyncSettings')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <SettingsRow
            icon="cloud-check"
            title="Storage Usage"
            subtitle="View space used by synced files"
            onPress={() => {}}
            colors={colors}
          />
        </View>
      </View>

      {/* App Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>APP</Text>
        <View style={[styles.sectionCard, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <SettingsRow
            icon="bell-outline"
            title="Notifications"
            subtitle="Sync alerts and updates"
            onPress={() => {}}
            colors={colors}
          />
          <Divider color={colors.border} />
          <SettingsRow
            icon="information-outline"
            title="About"
            subtitle="Version, licenses, support"
            onPress={() => navigation.navigate('About')}
            colors={colors}
          />
        </View>
      </View>

      {/* Disconnect Button */}
      <TouchableOpacity
        style={[styles.disconnectButton, {borderColor: '#EF4444'}]}
        onPress={handleLogout}>
        <Icon name="link-off" size={20} color="#EF4444" />
        <Text style={styles.disconnectText}>Disconnect Device</Text>
      </TouchableOpacity>

      <View style={styles.footer}>
        <Text style={[styles.footerText, {color: `${colors.text}40`}]}>
          NithronSync v1.0.0
        </Text>
      </View>
    </ScrollView>
  );
}

interface SettingsRowProps {
  icon: string;
  title: string;
  subtitle?: string;
  onPress: () => void;
  colors: {text: string; primary: string};
}

function SettingsRow({icon, title, subtitle, onPress, colors}: SettingsRowProps) {
  return (
    <TouchableOpacity style={styles.settingsRow} onPress={onPress}>
      <View style={[styles.rowIcon, {backgroundColor: `${colors.primary}15`}]}>
        <Icon name={icon} size={20} color={colors.primary} />
      </View>
      <View style={styles.rowContent}>
        <Text style={[styles.rowTitle, {color: colors.text}]}>{title}</Text>
        {subtitle && (
          <Text style={[styles.rowSubtitle, {color: `${colors.text}60`}]}>{subtitle}</Text>
        )}
      </View>
      <Icon name="chevron-right" size={20} color={`${colors.text}40`} />
    </TouchableOpacity>
  );
}

function Divider({color}: {color: string}) {
  return <View style={[styles.divider, {backgroundColor: color}]} />;
}

function getStatusIcon(status: SyncStatus): string {
  switch (status) {
    case 'syncing':
      return 'sync';
    case 'paused':
      return 'pause-circle';
    case 'error':
      return 'alert-circle';
    case 'offline':
      return 'cloud-off-outline';
    default:
      return 'check-circle';
  }
}

function getStatusColor(status: SyncStatus): string {
  switch (status) {
    case 'syncing':
      return '#2D7FF9';
    case 'paused':
      return '#F59E0B';
    case 'error':
      return '#EF4444';
    case 'offline':
      return '#6B7280';
    default:
      return '#10B981';
  }
}

function getStatusText(status: SyncStatus): string {
  switch (status) {
    case 'syncing':
      return 'Syncing...';
    case 'paused':
      return 'Sync Paused';
    case 'error':
      return 'Sync Error';
    case 'offline':
      return 'Offline';
    default:
      return 'Up to Date';
  }
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  statusCard: {
    margin: 16,
    padding: 16,
    borderRadius: 12,
    borderWidth: 1,
  },
  statusHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 16,
  },
  statusIcon: {
    width: 48,
    height: 48,
    borderRadius: 24,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  statusInfo: {
    flex: 1,
  },
  statusTitle: {
    fontSize: 18,
    fontWeight: '600',
    marginBottom: 2,
  },
  statusSubtitle: {
    fontSize: 13,
  },
  statusActions: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
  },
  syncButton: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingVertical: 10,
    borderRadius: 8,
    gap: 6,
  },
  syncButtonText: {
    color: '#FFFFFF',
    fontSize: 14,
    fontWeight: '600',
  },
  pauseToggle: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  pauseLabel: {
    fontSize: 14,
  },
  section: {
    marginTop: 8,
    paddingHorizontal: 16,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 8,
    marginLeft: 4,
  },
  sectionCard: {
    borderRadius: 12,
    borderWidth: 1,
    overflow: 'hidden',
  },
  settingsRow: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 14,
  },
  rowIcon: {
    width: 36,
    height: 36,
    borderRadius: 8,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 12,
  },
  rowContent: {
    flex: 1,
  },
  rowTitle: {
    fontSize: 15,
    fontWeight: '500',
  },
  rowSubtitle: {
    fontSize: 13,
    marginTop: 1,
  },
  divider: {
    height: 1,
    marginLeft: 62,
  },
  disconnectButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    marginHorizontal: 16,
    marginTop: 24,
    paddingVertical: 14,
    borderRadius: 10,
    borderWidth: 1.5,
    gap: 8,
  },
  disconnectText: {
    color: '#EF4444',
    fontSize: 15,
    fontWeight: '600',
  },
  footer: {
    alignItems: 'center',
    paddingVertical: 24,
  },
  footerText: {
    fontSize: 12,
  },
});

export default SettingsScreen;

