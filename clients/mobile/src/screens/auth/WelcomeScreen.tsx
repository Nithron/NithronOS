/**
 * Welcome Screen
 * Initial screen for unauthenticated users
 */

import React from 'react';
import {
  View,
  Text,
  StyleSheet,
  Image,
  TouchableOpacity,
  Dimensions,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {AuthStackParamList} from '../../navigation/RootNavigator';

const {width} = Dimensions.get('window');

type WelcomeNavigationProp = NativeStackNavigationProp<AuthStackParamList, 'Welcome'>;

export function WelcomeScreen(): React.JSX.Element {
  const navigation = useNavigation<WelcomeNavigationProp>();
  const {colors} = useTheme();

  return (
    <View style={[styles.container, {backgroundColor: colors.background}]}>
      {/* Logo and branding */}
      <View style={styles.header}>
        <View style={[styles.logoContainer, {backgroundColor: colors.primary}]}>
          <Icon name="cloud-sync" size={64} color="#FFFFFF" />
        </View>
        <Text style={[styles.title, {color: colors.text}]}>NithronSync</Text>
        <Text style={[styles.subtitle, {color: colors.text}]}>
          Sync your files across all devices
        </Text>
      </View>

      {/* Features list */}
      <View style={styles.features}>
        <FeatureItem
          icon="sync"
          title="Seamless Sync"
          description="Keep your files up-to-date everywhere"
          colors={colors}
        />
        <FeatureItem
          icon="lock"
          title="Secure & Private"
          description="Your data stays on your server"
          colors={colors}
        />
        <FeatureItem
          icon="cellphone-link"
          title="Cross-Platform"
          description="Works on all your devices"
          colors={colors}
        />
      </View>

      {/* Action buttons */}
      <View style={styles.actions}>
        <TouchableOpacity
          style={[styles.primaryButton, {backgroundColor: colors.primary}]}
          onPress={() => navigation.navigate('QRScanner')}>
          <Icon name="qrcode-scan" size={24} color="#FFFFFF" />
          <Text style={styles.primaryButtonText}>Scan QR Code</Text>
        </TouchableOpacity>

        <TouchableOpacity
          style={[styles.secondaryButton, {borderColor: colors.border}]}
          onPress={() => navigation.navigate('ManualSetup')}>
          <Icon name="cog" size={20} color={colors.primary} />
          <Text style={[styles.secondaryButtonText, {color: colors.primary}]}>
            Manual Setup
          </Text>
        </TouchableOpacity>
      </View>

      {/* Footer */}
      <Text style={[styles.footer, {color: colors.text}]}>
        Visit your NithronOS dashboard to get a QR code
      </Text>
    </View>
  );
}

interface FeatureItemProps {
  icon: string;
  title: string;
  description: string;
  colors: {text: string; primary: string; border: string};
}

function FeatureItem({icon, title, description, colors}: FeatureItemProps) {
  return (
    <View style={styles.featureItem}>
      <View style={[styles.featureIcon, {backgroundColor: `${colors.primary}20`}]}>
        <Icon name={icon} size={24} color={colors.primary} />
      </View>
      <View style={styles.featureText}>
        <Text style={[styles.featureTitle, {color: colors.text}]}>{title}</Text>
        <Text style={[styles.featureDescription, {color: `${colors.text}80`}]}>
          {description}
        </Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    paddingHorizontal: 24,
    paddingVertical: 48,
    justifyContent: 'space-between',
  },
  header: {
    alignItems: 'center',
    paddingTop: 32,
  },
  logoContainer: {
    width: 120,
    height: 120,
    borderRadius: 30,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 24,
    shadowColor: '#000',
    shadowOffset: {width: 0, height: 4},
    shadowOpacity: 0.2,
    shadowRadius: 8,
    elevation: 8,
  },
  title: {
    fontSize: 32,
    fontWeight: '700',
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 16,
    opacity: 0.7,
    textAlign: 'center',
  },
  features: {
    marginVertical: 32,
  },
  featureItem: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 20,
  },
  featureIcon: {
    width: 48,
    height: 48,
    borderRadius: 12,
    justifyContent: 'center',
    alignItems: 'center',
    marginRight: 16,
  },
  featureText: {
    flex: 1,
  },
  featureTitle: {
    fontSize: 16,
    fontWeight: '600',
    marginBottom: 2,
  },
  featureDescription: {
    fontSize: 14,
  },
  actions: {
    gap: 12,
  },
  primaryButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 16,
    borderRadius: 12,
    gap: 8,
  },
  primaryButtonText: {
    color: '#FFFFFF',
    fontSize: 18,
    fontWeight: '600',
  },
  secondaryButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 14,
    borderRadius: 12,
    borderWidth: 1.5,
    gap: 8,
  },
  secondaryButtonText: {
    fontSize: 16,
    fontWeight: '500',
  },
  footer: {
    textAlign: 'center',
    fontSize: 13,
    opacity: 0.6,
    marginTop: 16,
  },
});

export default WelcomeScreen;

