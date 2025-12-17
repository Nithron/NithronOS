/**
 * About Screen
 * App information and support
 */

import React from 'react';
import {
  View,
  Text,
  StyleSheet,
  ScrollView,
  TouchableOpacity,
  Linking,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

const APP_VERSION = '1.0.0';
const BUILD_NUMBER = '1';

export function AboutScreen(): React.JSX.Element {
  const {colors} = useTheme();

  const handleLink = (url: string) => {
    Linking.openURL(url);
  };

  return (
    <ScrollView style={[styles.container, {backgroundColor: colors.background}]}>
      {/* App Logo & Info */}
      <View style={styles.header}>
        <View style={[styles.logo, {backgroundColor: colors.primary}]}>
          <Icon name="cloud-sync" size={48} color="#FFFFFF" />
        </View>
        <Text style={[styles.appName, {color: colors.text}]}>NithronSync</Text>
        <Text style={[styles.version, {color: `${colors.text}60`}]}>
          Version {APP_VERSION} ({BUILD_NUMBER})
        </Text>
      </View>

      {/* Description */}
      <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
        <Text style={[styles.description, {color: `${colors.text}80`}]}>
          NithronSync is the official mobile client for NithronOS file synchronization.
          Keep your files in sync across all your devices with secure, private, self-hosted storage.
        </Text>
      </View>

      {/* Links Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>SUPPORT</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <LinkRow
            icon="web"
            title="Website"
            subtitle="nithron.com"
            onPress={() => handleLink('https://nithron.com')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <LinkRow
            icon="book-open-variant"
            title="Documentation"
            subtitle="View user guide"
            onPress={() => handleLink('https://docs.nithron.com/sync')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <LinkRow
            icon="discord"
            title="Community"
            subtitle="Join our Discord"
            onPress={() => handleLink('https://discord.gg/qzB37WS5AT')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <LinkRow
            icon="email-outline"
            title="Support"
            subtitle="hello@nithron.com"
            onPress={() => handleLink('mailto:hello@nithron.com')}
            colors={colors}
          />
        </View>
      </View>

      {/* Legal Section */}
      <View style={styles.section}>
        <Text style={[styles.sectionTitle, {color: `${colors.text}60`}]}>LEGAL</Text>
        <View style={[styles.card, {backgroundColor: colors.card, borderColor: colors.border}]}>
          <LinkRow
            icon="shield-lock-outline"
            title="Privacy Policy"
            onPress={() => handleLink('https://nithron.com/privacy')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <LinkRow
            icon="file-document-outline"
            title="Terms of Service"
            onPress={() => handleLink('https://nithron.com/terms')}
            colors={colors}
          />
          <Divider color={colors.border} />
          <LinkRow
            icon="license"
            title="Open Source Licenses"
            onPress={() => handleLink('https://nithron.com/licenses')}
            colors={colors}
          />
        </View>
      </View>

      {/* Footer */}
      <View style={styles.footer}>
        <Text style={[styles.footerText, {color: `${colors.text}40`}]}>
          © 2024 Nithron. All rights reserved.
        </Text>
        <Text style={[styles.footerText, {color: `${colors.text}30`}]}>
          Made with ♥ for the self-hosted community
        </Text>
      </View>
    </ScrollView>
  );
}

interface LinkRowProps {
  icon: string;
  title: string;
  subtitle?: string;
  onPress: () => void;
  colors: {text: string; primary: string};
}

function LinkRow({icon, title, subtitle, onPress, colors}: LinkRowProps) {
  return (
    <TouchableOpacity style={styles.linkRow} onPress={onPress}>
      <View style={[styles.iconContainer, {backgroundColor: `${colors.primary}15`}]}>
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

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  header: {
    alignItems: 'center',
    paddingVertical: 32,
  },
  logo: {
    width: 88,
    height: 88,
    borderRadius: 22,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 16,
  },
  appName: {
    fontSize: 24,
    fontWeight: '700',
    marginBottom: 4,
  },
  version: {
    fontSize: 14,
  },
  card: {
    marginHorizontal: 16,
    borderRadius: 12,
    borderWidth: 1,
    overflow: 'hidden',
  },
  description: {
    fontSize: 15,
    lineHeight: 22,
    padding: 16,
    textAlign: 'center',
  },
  section: {
    marginTop: 24,
    paddingHorizontal: 16,
  },
  sectionTitle: {
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 8,
    marginLeft: 4,
  },
  linkRow: {
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
  footer: {
    alignItems: 'center',
    paddingVertical: 32,
    paddingHorizontal: 16,
  },
  footerText: {
    fontSize: 12,
    marginBottom: 4,
  },
});

export default AboutScreen;

