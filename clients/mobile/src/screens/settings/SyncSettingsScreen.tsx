/**
 * Sync Settings Screen
 * Configure synchronization options
 */

import React, {useState, useEffect} from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  Switch,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import Slider from '@react-native-community/slider';

import {useSyncStore} from '../../stores/syncStore';

export function SyncSettingsScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const {config, updateConfig, refreshConfig} = useSyncStore();

  const [wifiOnly, setWifiOnly] = useState(config?.sync_on_wifi_only ?? true);
  const [meteredAllowed, setMeteredAllowed] = useState(config?.sync_on_metered ?? false);
  const [autoUploadPhotos, setAutoUploadPhotos] = useState(config?.auto_upload_photos ?? false);
  const [autoUploadVideos, setAutoUploadVideos] = useState(config?.auto_upload_videos ?? false);
  const [bandwidthLimit, setBandwidthLimit] = useState(config?.bandwidth_limit ?? 0);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    refreshConfig();
  }, [refreshConfig]);

  useEffect(() => {
    if (config) {
      setWifiOnly(config.sync_on_wifi_only);
      setMeteredAllowed(config.sync_on_metered);
      setAutoUploadPhotos(config.auto_upload_photos);
      setAutoUploadVideos(config.auto_upload_videos);
      setBandwidthLimit(config.bandwidth_limit || 0);
    }
  }, [config]);

  const handleSave = async (updates: Partial<typeof config>) => {
    setIsSaving(true);
    try {
      await updateConfig(updates as any);
    } catch (error) {
      Alert.alert('Error', 'Failed to save settings');
    } finally {
      setIsSaving(false);
    }
  };

  const handleWifiOnlyToggle = async (value: boolean) => {
    setWifiOnly(value);
    await handleSave({sync_on_wifi_only: value});
  };

  const handleMeteredToggle = async (value: boolean) => {
    setMeteredAllowed(value);
    await handleSave({sync_on_metered: value});
  };

  const handleAutoPhotosToggle = async (value: boolean) => {
    setAutoUploadPhotos(value);
    await handleSave({auto_upload_photos: value});
  };

  const handleAutoVideosToggle = async (value: boolean) => {
    setAutoUploadVideos(value);
    await handleSave({auto_upload_videos: value});
  };

  const handleBandwidthChange = async (value: number) => {
    const rounded = Math.round(value);
    setBandwidthLimit(rounded);
  };

  const handleBandwidthComplete = async (value: number) => {
    const rounded = Math.round(value);
    await handleSave({bandwidth_limit: rounded});
  };

  const formatBandwidth = (mbps: number): string => {
    if (mbps === 0) return 'Unlimited';
    return `${mbps} MB/s`;
  };

  return (
    <ScrollView style={[styles.container, {backgroundColor: colors.background}]}>
      {isSaving && (
        <View style={styles.savingIndicator}>
          <ActivityIndicator size="small" color={colors.primary} />
          <Text style={[styles.savingText, {color: colors.primary}]}>Saving...</Text>
        </View>
      )}

      {/* Network Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>NETWORK</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <SettingRow
            icon="wifi"
            title="Wi-Fi Only"
            subtitle="Only sync when connected to Wi-Fi"
            colors={colors}>
            <Switch
              value={wifiOnly}
              onValueChange={handleWifiOnlyToggle}
              trackColor={{false: colors.border, true: colors.primary}}
            />
          </SettingRow>
          <Divider color={colors.border} />
          <SettingRow
            icon="signal-cellular-3"
            title="Cellular Data"
            subtitle="Allow sync on metered connections"
            colors={colors}>
            <Switch
              value={meteredAllowed}
              onValueChange={handleMeteredToggle}
              trackColor={{false: colors.border, true: colors.primary}}
              disabled={wifiOnly}
            />
          </SettingRow>
        </View>
      </View>

      {/* Bandwidth Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>BANDWIDTH</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <View style={styles.bandwidthRow}>
            <View style={styles.bandwidthHeader}>
              <Icon name="speedometer" size={20} color={colors.primary} />
              <Text style={[styles.bandwidthTitle, {color: colors.text}]}>
                Upload Limit
              </Text>
            </View>
            <Text style={[styles.bandwidthValue, {color: colors.primary}]}>
              {formatBandwidth(bandwidthLimit)}
            </Text>
          </View>
          <Slider
            style={styles.slider}
            minimumValue={0}
            maximumValue={100}
            step={1}
            value={bandwidthLimit}
            onValueChange={handleBandwidthChange}
            onSlidingComplete={handleBandwidthComplete}
            minimumTrackTintColor={colors.primary}
            maximumTrackTintColor={colors.border}
            thumbTintColor={colors.primary}
          />
          <Text style={[styles.bandwidthHint, {color: `${colors.text}50`}]}>
            Set to 0 for unlimited bandwidth
          </Text>
        </View>
      </View>

      {/* Camera Upload Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>CAMERA UPLOAD</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <SettingRow
            icon="image"
            title="Auto-upload Photos"
            subtitle="Automatically sync photos to server"
            colors={colors}>
            <Switch
              value={autoUploadPhotos}
              onValueChange={handleAutoPhotosToggle}
              trackColor={{false: colors.border, true: colors.primary}}
            />
          </SettingRow>
          <Divider color={colors.border} />
          <SettingRow
            icon="video"
            title="Auto-upload Videos"
            subtitle="Automatically sync videos to server"
            colors={colors}>
            <Switch
              value={autoUploadVideos}
              onValueChange={handleAutoVideosToggle}
              trackColor={{false: colors.border, true: colors.primary}}
            />
          </SettingRow>
        </View>
        <Text style={[styles.sectionHint, {color: `${colors.text}50`}]}>
          Photos and videos will be uploaded to your default sync share
        </Text>
      </View>

      {/* Storage Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>STORAGE</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <TouchableOpacity
            style={styles.storageRow}
            onPress={() => Alert.alert('Clear Cache', 'Cache cleared successfully')}>
            <View style={[styles.iconContainer, {backgroundColor: `${colors.primary}15`}]}>
              <Icon name="cached" size={20} color={colors.primary} />
            </View>
            <View style={styles.rowContent}>
              <Text style={[styles.rowTitle, {color: colors.text}]}>Clear Cache</Text>
              <Text style={[styles.rowSubtitle, {color: `${colors.text}60`}]}>
                Free up space by clearing temporary files
              </Text>
            </View>
            <Icon name="chevron-right" size={20} color={`${colors.text}40`} />
          </TouchableOpacity>
        </View>
      </View>
    </ScrollView>
  );
}

interface SettingRowProps {
  icon: string;
  title: string;
  subtitle: string;
  colors: {text: string; primary: string};
  children: React.ReactNode;
}

function SettingRow({icon, title, subtitle, colors, children}: SettingRowProps) {
  return (
    <View style={styles.settingRow}>
      <View style={[styles.iconContainer, {backgroundColor: `${colors.primary}15`}]}>
        <Icon name={icon} size={20} color={colors.primary} />
      </View>
      <View style={styles.rowContent}>
        <Text style={[styles.rowTitle, {color: colors.text}]}>{title}</Text>
        <Text style={[styles.rowSubtitle, {color: `${colors.text}60`}]}>{subtitle}</Text>
      </View>
      {children}
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
  savingIndicator: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 8,
    gap: 8,
  },
  savingText: {
    fontSize: 13,
    fontWeight: '500',
  },
  section: {
    paddingHorizontal: 16,
    paddingTop: 20,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 8,
    marginLeft: 4,
  },
  sectionHint: {
    fontSize: 13,
    marginTop: 8,
    marginLeft: 4,
  },
  card: {
    borderRadius: 12,
    borderWidth: 1,
    overflow: 'hidden',
  },
  settingRow: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 14,
  },
  storageRow: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 14,
  },
  iconContainer: {
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
  bandwidthRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    padding: 14,
    paddingBottom: 8,
  },
  bandwidthHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  bandwidthTitle: {
    fontSize: 15,
    fontWeight: '500',
  },
  bandwidthValue: {
    fontSize: 15,
    fontWeight: '600',
  },
  slider: {
    marginHorizontal: 14,
    marginBottom: 4,
  },
  bandwidthHint: {
    fontSize: 12,
    paddingHorizontal: 14,
    paddingBottom: 12,
  },
});

export default SyncSettingsScreen;

