/**
 * Welcome Screen
 * The first screen of an unpaired phone: the name, what the app does, and
 * the way to pair it.
 */

import React from 'react';
import {ScrollView, Text, View} from 'react-native';
import {useNavigation} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import {useSafeAreaInsets} from 'react-native-safe-area-context';

import {AuthStackParamList} from '../../navigation/RootNavigator';
import {Button} from '../../components/Button';
import {PairingLinkIcon, ScanCodeIcon} from '../../components/icons';
import {PixelText} from '../../components/pixel';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';

type WelcomeNavigationProp = NativeStackNavigationProp<
  AuthStackParamList,
  'Welcome'
>;

const FACTS = [
  'Browse the shares on your nOS machine.',
  'Files stay on nOS: the app does not copy them to this phone.',
  'Pair with the QR code nOS shows for a new device.',
];

export function WelcomeScreen(): React.JSX.Element {
  const navigation = useNavigation<WelcomeNavigationProp>();
  const styles = useThemedStyles(welcomeStyles);
  const insets = useSafeAreaInsets();

  return (
    <ScrollView
      style={styles.screen}
      contentContainerStyle={[
        styles.content,
        {
          paddingTop: insets.top + metrics.space6,
          paddingBottom: insets.bottom + metrics.space5,
        },
      ]}>
      <View style={styles.name}>
        <PixelText size={32} accessibilityRole="header">
          NithronSync
        </PixelText>
        <Text style={styles.lead}>Your nOS shares, on your phone.</Text>
      </View>

      <View style={styles.facts}>
        {FACTS.map(fact => (
          <Text key={fact} style={styles.fact}>
            {fact}
          </Text>
        ))}
      </View>

      <View style={styles.actions}>
        <Button
          kind="primary"
          icon={ScanCodeIcon}
          label="Scan the QR code"
          onPress={() => navigation.navigate('QRScanner')}
          block
        />
        <Button
          icon={PairingLinkIcon}
          label="Paste the pairing link"
          onPress={() => navigation.navigate('ManualSetup')}
          block
        />
        <Text style={styles.footer}>
          Add this phone in nOS under Sync → Devices.
        </Text>
      </View>
    </ScrollView>
  );
}

const welcomeStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {
    flexGrow: 1,
    justifyContent: 'space-between' as const,
    gap: metrics.space6,
  },
  name: {paddingHorizontal: metrics.gutter, gap: metrics.space3},
  lead: {...type.body, color: p.ink2},
  facts: {borderTopWidth: metrics.hair, borderTopColor: p.hairline},
  fact: {
    ...type.small,
    color: p.ink,
    paddingHorizontal: metrics.gutter,
    paddingVertical: metrics.space3,
    borderBottomWidth: metrics.hair,
    borderBottomColor: p.hairline,
  },
  actions: {paddingHorizontal: metrics.gutter, gap: metrics.space3},
  footer: {...type.fine, color: p.ink2, marginTop: metrics.space2},
});

export default WelcomeScreen;
