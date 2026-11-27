/**
 * Scans the pairing QR code the nOS web interface shows for a new device.
 */

import React, {useCallback, useRef, useState} from 'react';
import {
  ActivityIndicator,
  Dimensions,
  Linking,
  StyleSheet,
  Text,
  TouchableOpacity,
  View,
} from 'react-native';
import {useIsFocused, useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';
import {
  Camera,
  Code,
  useCameraDevice,
  useCameraPermission,
  useCodeScanner,
} from 'react-native-vision-camera';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {AuthStackParamList} from '../../navigation/RootNavigator';
import {explainBadLink, PAIRING_STEPS} from './pairingHelp';

const {width} = Dimensions.get('window');
const SCAN_AREA_SIZE = Math.min(width * 0.7, 280);

type QRScannerNavigationProp = NativeStackNavigationProp<
  AuthStackParamList,
  'QRScanner'
>;

export function QRScannerScreen(): React.JSX.Element {
  const navigation = useNavigation<QRScannerNavigationProp>();
  const {colors} = useTheme();
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

  if (!hasPermission) {
    return (
      <View
        style={[styles.messageContainer, {backgroundColor: colors.background}]}>
        <Icon name="camera-off" size={64} color={colors.text} />
        <Text style={[styles.messageTitle, {color: colors.text}]}>
          Camera access needed
        </Text>
        <Text style={[styles.messageText, {color: `${colors.text}99`}]}>
          {wasDenied
            ? 'Camera access is off for NithronSync. Turn it on in the system settings, or paste the pairing link instead.'
            : 'NithronSync uses the camera only to read the pairing QR code.'}
        </Text>
        <TouchableOpacity
          accessibilityRole="button"
          style={[styles.primaryButton, {backgroundColor: colors.primary}]}
          onPress={wasDenied ? () => Linking.openSettings() : askForCamera}>
          <Text style={styles.primaryButtonText}>
            {wasDenied ? 'Open settings' : 'Allow camera'}
          </Text>
        </TouchableOpacity>
        <PasteLinkButton
          color={colors.primary}
          onPress={() => navigation.navigate('ManualSetup')}
        />
      </View>
    );
  }

  if (!device) {
    return (
      <View
        style={[styles.messageContainer, {backgroundColor: colors.background}]}>
        <Icon name="camera-off" size={64} color={colors.text} />
        <Text style={[styles.messageTitle, {color: colors.text}]}>
          No camera found
        </Text>
        <PasteLinkButton
          color={colors.primary}
          onPress={() => navigation.navigate('ManualSetup')}
        />
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
        <View style={styles.overlayDark} />
        <View style={styles.overlayMiddle}>
          <View style={styles.overlayDark} />
          <View style={[styles.scanFrame, {borderColor: colors.primary}]}>
            {isPairing && (
              <View style={styles.pairingContainer}>
                <ActivityIndicator size="large" color={colors.primary} />
                <Text style={styles.pairingText}>Pairing…</Text>
              </View>
            )}
          </View>
          <View style={styles.overlayDark} />
        </View>
        <View style={[styles.overlayDark, styles.bottomArea]}>
          {failure ? (
            <View style={styles.failure} accessibilityLiveRegion="polite">
              <Icon name="alert-circle" size={20} color="#FFFFFF" />
              <Text style={styles.failureText}>{failure}</Text>
              <TouchableOpacity
                accessibilityRole="button"
                style={[
                  styles.primaryButton,
                  {backgroundColor: colors.primary},
                ]}
                onPress={scanAgain}>
                <Text style={styles.primaryButtonText}>Scan again</Text>
              </TouchableOpacity>
            </View>
          ) : (
            <Text style={styles.instructionText}>{PAIRING_STEPS}</Text>
          )}
          <PasteLinkButton
            color="#FFFFFF"
            onPress={() => navigation.navigate('ManualSetup')}
          />
        </View>
      </View>
    </View>
  );
}

function PasteLinkButton({
  color,
  onPress,
}: {
  color: string;
  onPress: () => void;
}) {
  return (
    <TouchableOpacity
      accessibilityRole="button"
      style={styles.linkButton}
      onPress={onPress}>
      <Icon name="link-variant" size={18} color={color} />
      <Text style={[styles.linkButtonText, {color}]}>
        Paste the pairing link
      </Text>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#000000',
  },
  overlay: {
    ...StyleSheet.absoluteFillObject,
  },
  overlayDark: {
    flex: 1,
    backgroundColor: 'rgba(0, 0, 0, 0.6)',
  },
  overlayMiddle: {
    flexDirection: 'row',
    height: SCAN_AREA_SIZE,
  },
  scanFrame: {
    width: SCAN_AREA_SIZE,
    height: SCAN_AREA_SIZE,
    borderWidth: 3,
    borderRadius: 16,
    overflow: 'hidden',
  },
  pairingContainer: {
    ...StyleSheet.absoluteFillObject,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
  },
  pairingText: {
    color: '#FFFFFF',
    marginTop: 12,
    fontSize: 14,
  },
  bottomArea: {
    flex: 1.4,
    padding: 24,
    alignItems: 'center',
  },
  instructionText: {
    color: '#FFFFFF',
    fontSize: 15,
    lineHeight: 21,
    textAlign: 'center',
  },
  failure: {
    alignItems: 'center',
    gap: 10,
  },
  failureText: {
    color: '#FFFFFF',
    fontSize: 15,
    lineHeight: 21,
    textAlign: 'center',
  },
  messageContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  messageTitle: {
    fontSize: 20,
    fontWeight: '600',
    marginTop: 16,
    marginBottom: 8,
  },
  messageText: {
    fontSize: 14,
    lineHeight: 20,
    textAlign: 'center',
    marginBottom: 24,
  },
  primaryButton: {
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
    minHeight: 48,
    justifyContent: 'center',
  },
  primaryButtonText: {
    color: '#FFFFFF',
    fontSize: 16,
    fontWeight: '600',
  },
  linkButton: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    marginTop: 16,
    minHeight: 48,
    paddingHorizontal: 12,
  },
  linkButtonText: {
    fontSize: 15,
    fontWeight: '500',
  },
});

export default QRScannerScreen;
