/**
 * Settings: the server this phone is paired with, this device, and
 * unpairing.
 */

import React from 'react';
import {
  ActivityIndicator,
  Alert,
  ScrollView,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {useSyncStore} from '../../stores/syncStore';
import {SettingsStackParamList} from '../../navigation/RootNavigator';
import {APP_VERSION} from '../../version';

type SettingsNavigationProp = NativeStackNavigationProp<
  SettingsStackParamList,
  'SettingsMain'
>;

const CONNECTED = '#10B981';
const PROBLEM = '#EF4444';

export function SettingsScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const navigation = useNavigation<SettingsNavigationProp>();
  const {device, serverUrl, error, isBusy, unpair, refreshDevice} =
    useAuthStore();
  const resetData = useSyncStore(s => s.reset);

  const statusColor = error ? PROBLEM : CONNECTED;

  const doUnpair = async () => {
    try {
      const {revoked} = await unpair();
      resetData();
      if (!revoked) {
        Alert.alert(
          'Unpaired on this phone',
          "The server couldn't be told, so the device is still listed in nOS. Remove it under Settings → Sync → Devices.",
        );
      }
    } catch (err) {
      Alert.alert("Couldn't unpair", errorMessage(err));
    }
  };

  const handleUnpair = () => {
    Alert.alert(
      'Unpair this phone',
      'This removes the device from nOS and forgets the pairing here. To use it again, pair it with a new QR code.',
      [
        {text: 'Cancel', style: 'cancel'},
        {text: 'Unpair', style: 'destructive', onPress: doUnpair},
      ],
    );
  };

  return (
    <ScrollView
      style={[styles.container, {backgroundColor: colors.background}]}>
      <View
        style={[
          styles.statusCard,
          {backgroundColor: colors.card, borderColor: colors.border},
        ]}>
        <View style={styles.statusHeader}>
          <View
            style={[styles.statusIcon, {backgroundColor: `${statusColor}20`}]}>
            <Icon
              name={error ? 'cloud-alert' : 'check-circle'}
              size={24}
              color={statusColor}
            />
          </View>
          <View style={styles.statusInfo}>
            <Text style={[styles.statusTitle, {color: colors.text}]}>
              {error ? "Can't reach the server" : 'Paired'}
            </Text>
            <Text
              style={[styles.statusSubtitle, {color: `${colors.text}99`}]}
              numberOfLines={3}>
              {error ?? serverUrl ?? ''}
            </Text>
          </View>
          {error && (
            <TouchableOpacity
              accessibilityRole="button"
              accessibilityLabel="Try again"
              style={styles.retry}
              onPress={refreshDevice}>
              <Icon name="refresh" size={22} color={colors.primary} />
            </TouchableOpacity>
          )}
        </View>
      </View>

      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}99`}]}>
          DEVICE
        </Text>
        <View
          style={[
            styles.sectionCard,
            {backgroundColor: colors.card, borderColor: colors.border},
          ]}>
          <SettingsRow
            icon="cellphone"
            title={device?.device_name ?? 'This Device'}
            subtitle="Name, sync statistics"
            onPress={() => navigation.navigate('DeviceSettings')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <SettingsRow
            icon="information-outline"
            title="About"
            subtitle="Version, links"
            onPress={() => navigation.navigate('About')}
            colors={colors}
          />
        </View>
      </View>

      <TouchableOpacity
        accessibilityRole="button"
        accessibilityState={{disabled: isBusy}}
        style={[styles.unpairButton, {borderColor: PROBLEM}]}
        onPress={handleUnpair}
        disabled={isBusy}>
        {isBusy ? (
          <ActivityIndicator color={PROBLEM} />
        ) : (
          <>
            <Icon name="link-off" size={20} color={PROBLEM} />
            <Text style={styles.unpairText}>Unpair This Phone</Text>
          </>
        )}
      </TouchableOpacity>

      <View style={styles.footer}>
        <Text style={[styles.footerText, {color: `${colors.text}80`}]}>
          NithronSync {APP_VERSION}
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

function SettingsRow({
  icon,
  title,
  subtitle,
  onPress,
  colors,
}: SettingsRowProps) {
  return (
    <TouchableOpacity
      accessibilityRole="button"
      style={styles.settingsRow}
      onPress={onPress}>
      <View style={[styles.rowIcon, {backgroundColor: `${colors.primary}15`}]}>
        <Icon name={icon} size={20} color={colors.primary} />
      </View>
      <View style={styles.rowContent}>
        <Text style={[styles.rowTitle, {color: colors.text}]}>{title}</Text>
        {subtitle && (
          <Text style={[styles.rowSubtitle, {color: `${colors.text}99`}]}>
            {subtitle}
          </Text>
        )}
      </View>
      <Icon name="chevron-right" size={20} color={`${colors.text}40`} />
    </TouchableOpacity>
  );
}

function Divider({color}: {color: string}) {
  return <View style={[styles.divider, {backgroundColor: color}]} />;
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
  retry: {
    padding: 12,
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
    minHeight: 56,
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
  unpairButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    marginHorizontal: 16,
    marginTop: 24,
    paddingVertical: 14,
    borderRadius: 10,
    borderWidth: 1.5,
    gap: 8,
    minHeight: 52,
  },
  unpairText: {
    color: PROBLEM,
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
