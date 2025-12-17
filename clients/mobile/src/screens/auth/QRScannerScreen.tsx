/**
 * QR Scanner Screen
 * Scan QR code from NithronOS dashboard for quick setup
 */

import React, {useState, useCallback} from 'react';
import {
  View,
  Text,
  StyleSheet,
  TouchableOpacity,
  Alert,
  ActivityIndicator,
  Dimensions,
} from 'react-native';
import {useNavigation, useTheme} from '@react-navigation/native';
import {NativeStackNavigationProp} from '@react-navigation/native-stack';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {useAuthStore} from '../../stores/authStore';
import {AuthStackParamList} from '../../navigation/RootNavigator';

const {width, height} = Dimensions.get('window');
const SCAN_AREA_SIZE = Math.min(width * 0.7, 280);

type QRScannerNavigationProp = NativeStackNavigationProp<AuthStackParamList, 'QRScanner'>;

// Note: In a real implementation, this would use react-native-qrcode-scanner
// For now, we'll create a mock UI that simulates the scanner

export function QRScannerScreen(): React.JSX.Element {
  const navigation = useNavigation<QRScannerNavigationProp>();
  const {colors} = useTheme();
  const {registerFromQRCode, isLoading, error} = useAuthStore();

  const [isScanning, setIsScanning] = useState(true);
  const [hasPermission, setHasPermission] = useState(true);

  // Handle QR code scan
  const handleScan = useCallback(
    async (data: string) => {
      if (!isScanning || isLoading) return;
      
      setIsScanning(false);

      try {
        // Validate QR code format
        if (!data.startsWith('nithronos://')) {
          throw new Error('Invalid QR code. Please scan a NithronOS setup QR code.');
        }

        await registerFromQRCode(data);
        // Navigation will happen automatically via RootNavigator
      } catch (err) {
        const message = (err as Error).message || 'Failed to connect';
        Alert.alert('Connection Failed', message, [
          {
            text: 'Try Again',
            onPress: () => setIsScanning(true),
          },
          {
            text: 'Manual Setup',
            onPress: () => navigation.navigate('ManualSetup'),
          },
        ]);
      }
    },
    [isScanning, isLoading, registerFromQRCode, navigation]
  );

  // Mock scan for development/testing
  const handleMockScan = () => {
    // In development, allow testing with a mock QR code
    Alert.prompt(
      'Enter QR Code Data',
      'Paste the nithronos:// URL for testing:',
      [
        {text: 'Cancel', style: 'cancel'},
        {
          text: 'Connect',
          onPress: (text) => {
            if (text) {
              handleScan(text);
            }
          },
        },
      ],
      'plain-text',
      'nithronos://sync?server=https://your-server.com&token=...'
    );
  };

  if (!hasPermission) {
    return (
      <View style={[styles.container, {backgroundColor: colors.background}]}>
        <View style={styles.permissionContainer}>
          <Icon name="camera-off" size={64} color={colors.text} />
          <Text style={[styles.permissionTitle, {color: colors.text}]}>
            Camera Permission Required
          </Text>
          <Text style={[styles.permissionText, {color: `${colors.text}80`}]}>
            Please allow camera access to scan QR codes
          </Text>
          <TouchableOpacity
            style={[styles.permissionButton, {backgroundColor: colors.primary}]}
            onPress={() => {
              // Would open app settings
              setHasPermission(true);
            }}>
            <Text style={styles.permissionButtonText}>Open Settings</Text>
          </TouchableOpacity>
        </View>
      </View>
    );
  }

  return (
    <View style={[styles.container, {backgroundColor: '#000'}]}>
      {/* Camera preview area (mock) */}
      <View style={styles.cameraContainer}>
        {/* Scan frame overlay */}
        <View style={styles.overlay}>
          {/* Top dark area */}
          <View style={styles.overlayDark} />
          
          {/* Middle row with scan area */}
          <View style={styles.overlayMiddle}>
            <View style={styles.overlayDark} />
            
            {/* Scan frame */}
            <View style={[styles.scanFrame, {borderColor: colors.primary}]}>
              {/* Corner decorations */}
              <View style={[styles.corner, styles.cornerTL, {borderColor: colors.primary}]} />
              <View style={[styles.corner, styles.cornerTR, {borderColor: colors.primary}]} />
              <View style={[styles.corner, styles.cornerBL, {borderColor: colors.primary}]} />
              <View style={[styles.corner, styles.cornerBR, {borderColor: colors.primary}]} />
              
              {/* Scanning line animation would go here */}
              {isScanning && !isLoading && (
                <View style={[styles.scanLine, {backgroundColor: colors.primary}]} />
              )}
              
              {/* Loading indicator */}
              {isLoading && (
                <View style={styles.loadingContainer}>
                  <ActivityIndicator size="large" color={colors.primary} />
                  <Text style={styles.loadingText}>Connecting...</Text>
                </View>
              )}
            </View>
            
            <View style={styles.overlayDark} />
          </View>
          
          {/* Bottom dark area */}
          <View style={styles.overlayDark} />
        </View>
      </View>

      {/* Instructions */}
      <View style={styles.instructions}>
        <Text style={styles.instructionText}>
          Point your camera at the QR code from your NithronOS dashboard
        </Text>
        
        <View style={styles.helpRow}>
          <Icon name="information" size={16} color="#FFFFFF80" />
          <Text style={styles.helpText}>
            Go to Settings → Sync → Devices → Register Device
          </Text>
        </View>
      </View>

      {/* Bottom actions */}
      <View style={styles.bottomActions}>
        <TouchableOpacity
          style={[styles.actionButton, {backgroundColor: `${colors.primary}20`}]}
          onPress={handleMockScan}>
          <Icon name="code-tags" size={20} color={colors.primary} />
          <Text style={[styles.actionButtonText, {color: colors.primary}]}>
            Enter Code Manually
          </Text>
        </TouchableOpacity>

        <TouchableOpacity
          style={[styles.actionButton, {backgroundColor: `${colors.primary}20`}]}
          onPress={() => navigation.navigate('ManualSetup')}>
          <Icon name="cog" size={20} color={colors.primary} />
          <Text style={[styles.actionButtonText, {color: colors.primary}]}>
            Manual Setup
          </Text>
        </TouchableOpacity>
      </View>

      {/* Error display */}
      {error && (
        <View style={styles.errorContainer}>
          <Icon name="alert-circle" size={20} color="#EF4444" />
          <Text style={styles.errorText}>{error}</Text>
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  cameraContainer: {
    flex: 1,
    position: 'relative',
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
    borderWidth: 2,
    borderRadius: 16,
    position: 'relative',
    overflow: 'hidden',
  },
  corner: {
    position: 'absolute',
    width: 24,
    height: 24,
    borderWidth: 3,
  },
  cornerTL: {
    top: -2,
    left: -2,
    borderRightWidth: 0,
    borderBottomWidth: 0,
    borderTopLeftRadius: 14,
  },
  cornerTR: {
    top: -2,
    right: -2,
    borderLeftWidth: 0,
    borderBottomWidth: 0,
    borderTopRightRadius: 14,
  },
  cornerBL: {
    bottom: -2,
    left: -2,
    borderRightWidth: 0,
    borderTopWidth: 0,
    borderBottomLeftRadius: 14,
  },
  cornerBR: {
    bottom: -2,
    right: -2,
    borderLeftWidth: 0,
    borderTopWidth: 0,
    borderBottomRightRadius: 14,
  },
  scanLine: {
    position: 'absolute',
    left: 16,
    right: 16,
    height: 2,
    top: '50%',
    opacity: 0.8,
  },
  loadingContainer: {
    ...StyleSheet.absoluteFillObject,
    justifyContent: 'center',
    alignItems: 'center',
    backgroundColor: 'rgba(0, 0, 0, 0.5)',
  },
  loadingText: {
    color: '#FFFFFF',
    marginTop: 12,
    fontSize: 14,
  },
  instructions: {
    padding: 24,
    alignItems: 'center',
  },
  instructionText: {
    color: '#FFFFFF',
    fontSize: 16,
    textAlign: 'center',
    marginBottom: 12,
  },
  helpRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  helpText: {
    color: '#FFFFFF80',
    fontSize: 13,
  },
  bottomActions: {
    flexDirection: 'row',
    paddingHorizontal: 16,
    paddingBottom: 32,
    gap: 12,
  },
  actionButton: {
    flex: 1,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 14,
    borderRadius: 10,
    gap: 8,
  },
  actionButtonText: {
    fontSize: 14,
    fontWeight: '500',
  },
  errorContainer: {
    position: 'absolute',
    top: 100,
    left: 24,
    right: 24,
    flexDirection: 'row',
    alignItems: 'center',
    backgroundColor: 'rgba(239, 68, 68, 0.9)',
    padding: 12,
    borderRadius: 8,
    gap: 8,
  },
  errorText: {
    color: '#FFFFFF',
    flex: 1,
    fontSize: 14,
  },
  permissionContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
    padding: 32,
  },
  permissionTitle: {
    fontSize: 20,
    fontWeight: '600',
    marginTop: 16,
    marginBottom: 8,
  },
  permissionText: {
    fontSize: 14,
    textAlign: 'center',
    marginBottom: 24,
  },
  permissionButton: {
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: 8,
  },
  permissionButtonText: {
    color: '#FFFFFF',
    fontSize: 16,
    fontWeight: '500',
  },
});

export default QRScannerScreen;

