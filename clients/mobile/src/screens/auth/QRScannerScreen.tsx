/**
 * Scans the pairing QR code the nOS web interface shows for a new device.
 */

import React, {useCallback, useRef, useState} from 'react';
import {Dimensions, Linking, StyleSheet, Text, View} from 'react-native';
import {useIsFocused, useNavigation} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import {
  Camera,
  Code,
  useCameraDevice,
  useCameraPermission,
  useCodeScanner,
} from 'react-native-vision-camera';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {AuthStackParamList} from '../../navigation/RootNavigator';
import {Button} from '../../components/Button';
import {PairingLinkIcon} from '../../components/icons';
import {StatusMark, Working} from '../../components/pixel';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';
import {explainBadLink, PAIRING_STEPS} from './pairingHelp';

const SCAN_AREA_SHARE = 0.7;
const SCAN_AREA_MAX = 280;
const {width} = Dimensions.get('window');
const SCAN_AREA_SIZE = Math.min(width * SCAN_AREA_SHARE, SCAN_AREA_MAX);

type QRScannerNavigationProp = NativeStackNavigationProp<
  AuthStackParamList,
  'QRScanner'
>;

export function QRScannerScreen(): React.JSX.Element {
  const navigation = useNavigation<QRScannerNavigationProp>();
  const styles = useThemedStyles(scannerStyles);
  const isFocused = useIsFocused();
  const {hasPermission, requestPermission} = useCameraPermission();
  const device = useCameraDevice('back');
  const pairWithLink = useAuthStore(s => s.pairWithLink);

  const [isPairing, setIsPairing] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  const [wasDenied, setWasDenied] = useState(false);
  // Codes keep arriving while a scan is being handled.
  const handling = useRef(false);

  const onCodeScanned = useCallback(
    async (codes: Code[]) => {
      const link = codes.find(c => c.value)?.value;
      if (!link || handling.current) {
        return;
      }
      handling.current = true;
      setIsPairing(true);
      setFailure(null);
      try {
        await pairWithLink(link);
        // Paired: the navigator switches to the app.
      } catch (error) {
        setFailure(explainBadLink(link, errorMessage(error)));
      } finally {
        setIsPairing(false);
      }
    },
    [pairWithLink],
  );

  const codeScanner = useCodeScanner({codeTypes: ['qr'], onCodeScanned});

  const scanAgain = () => {
    setFailure(null);
    handling.current = false;
  };

  const askForCamera = async () => {
    const granted = await requestPermission();
    setWasDenied(!granted);
  };

  const pasteLink = (
    <Button
      icon={PairingLinkIcon}
      label="Paste the pairing link"
      onPress={() => navigation.navigate('ManualSetup')}
      block
    />
  );

  if (!hasPermission) {
    return (
      <View style={styles.message}>
        <Text accessibilityRole="header" style={styles.messageTitle}>
          Camera access needed
        </Text>
        <Text style={styles.messageText}>
          {wasDenied
            ? 'Camera access is off for NithronSync. Turn it on in the system settings, or paste the pairing link instead.'
            : 'NithronSync uses the camera only to read the pairing QR code.'}
        </Text>
        <View style={styles.messageActions}>
          <Button
            kind="primary"
            label={wasDenied ? 'Open settings' : 'Allow camera'}
            onPress={wasDenied ? () => Linking.openSettings() : askForCamera}
            block
          />
          {pasteLink}
        </View>
      </View>
    );
  }

  if (!device) {
    return (
      <View style={styles.message}>
        <Text accessibilityRole="header" style={styles.messageTitle}>
          No camera found
        </Text>
        <Text style={styles.messageText}>
          Paste the pairing link instead.
        </Text>
        <View style={styles.messageActions}>{pasteLink}</View>
      </View>
    );
  }

  const scanning = !isPairing && failure === null;

  return (
    <View style={styles.container}>
      <Camera
        style={StyleSheet.absoluteFill}
        device={device}
        isActive={isFocused && scanning}
        codeScanner={codeScanner}
      />

      <View style={styles.overlay} pointerEvents="box-none">
        <View style={styles.dim} />
        <View style={styles.middle}>
          <View style={styles.dim} />
          <View style={styles.frame}>
            {isPairing && (
              <View style={styles.pairing}>
                <Working>Pairing</Working>
              </View>
            )}
          </View>
          <View style={styles.dim} />
        </View>
        <View style={[styles.dim, styles.bottomArea]}>
          {failure ? (
            <View style={styles.failure} accessibilityLiveRegion="polite">
              <StatusMark state="failed">{failure}</StatusMark>
              <Button
                kind="primary"
                label="Scan again"
                onPress={scanAgain}
                block
              />
            </View>
          ) : (
            <Text style={styles.instruction}>{PAIRING_STEPS}</Text>
          )}
          {pasteLink}
        </View>
      </View>
    </View>
  );
}

const scannerStyles = (p: Palette) => ({
  container: {flex: 1, backgroundColor: p.page},
  overlay: {...StyleSheet.absoluteFillObject},
  // Outside the frame the picture lies under the ground at 70 percent.
  dim: {flex: 1, backgroundColor: p.scrim},
  middle: {flexDirection: 'row' as const, height: SCAN_AREA_SIZE},
  frame: {
    width: SCAN_AREA_SIZE,
    height: SCAN_AREA_SIZE,
    borderWidth: metrics.focus,
    borderColor: p.main,
  },
  pairing: {
    ...StyleSheet.absoluteFillObject,
    justifyContent: 'center' as const,
    paddingHorizontal: metrics.gutter,
    backgroundColor: p.scrim,
  },
  bottomArea: {
    flex: 1.4,
    padding: metrics.gutter,
    gap: metrics.space4,
  },
  instruction: {...type.small, color: p.ink},
  failure: {gap: metrics.space3},
  message: {
    flex: 1,
    justifyContent: 'center' as const,
    padding: metrics.gutter,
    gap: metrics.space3,
    backgroundColor: p.page,
  },
  messageTitle: {...type.title, color: p.ink},
  messageText: {...type.small, color: p.ink2},
  messageActions: {gap: metrics.space3, marginTop: metrics.space3},
});

export default QRScannerScreen;
