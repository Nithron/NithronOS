/**
 * Manual Setup Screen
 * Configure server connection manually
 */

import React, {useState} from 'react';
import {
  View,
  Text,
  StyleSheet,
  TextInput,
  TouchableOpacity,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
  Alert,
  ActivityIndicator,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {useAuthStore} from '../../stores/authStore';

export function ManualSetupScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const {connectToServer, registerDevice, isLoading, error, clearError} = useAuthStore();

  // Form state
  const [step, setStep] = useState<'server' | 'device'>('server');
  const [serverUrl, setServerUrl] = useState('');
  const [deviceName, setDeviceName] = useState('');
  const [isConnecting, setIsConnecting] = useState(false);

  // Validate URL
  const isValidUrl = (url: string): boolean => {
    try {
      const parsed = new URL(url);
      return parsed.protocol === 'https:' || parsed.protocol === 'http:';
    } catch {
      return false;
    }
  };

  // Connect to server
  const handleConnect = async () => {
    clearError();
    
    // Normalize URL
    let url = serverUrl.trim();
    if (!url.startsWith('http://') && !url.startsWith('https://')) {
      url = `https://${url}`;
    }

    if (!isValidUrl(url)) {
      Alert.alert('Invalid URL', 'Please enter a valid server URL');
      return;
    }

    setIsConnecting(true);
    try {
      await connectToServer(url);
      // Generate default device name
      setDeviceName(`Mobile Device (${Platform.OS})`);
      setStep('device');
    } catch (err) {
      Alert.alert(
        'Connection Failed',
        (err as Error).message || 'Could not connect to server. Please check the URL and try again.'
      );
    } finally {
      setIsConnecting(false);
    }
  };

  // Register device
  const handleRegister = async () => {
    if (!deviceName.trim()) {
      Alert.alert('Device Name Required', 'Please enter a name for this device');
      return;
    }

    try {
      await registerDevice({
        name: deviceName.trim(),
        type: 'mobile',
        platform: Platform.OS as 'ios' | 'android',
        osVersion: Platform.Version.toString(),
      });
      // Navigation will happen automatically
    } catch (err) {
      Alert.alert(
        'Registration Failed',
        (err as Error).message || 'Could not register device. Please try again.'
      );
    }
  };

  return (
    <KeyboardAvoidingView
      style={{flex: 1}}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView
        style={[styles.container, {backgroundColor: colors.background}]}
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled">
        
        {/* Step indicator */}
        <View style={styles.stepIndicator}>
          <View style={styles.stepRow}>
            <StepBadge
              number={1}
              label="Server"
              active={step === 'server'}
              completed={step === 'device'}
              colors={colors}
            />
            <View style={[styles.stepLine, {backgroundColor: step === 'device' ? colors.primary : colors.border}]} />
            <StepBadge
              number={2}
              label="Device"
              active={step === 'device'}
              completed={false}
              colors={colors}
            />
          </View>
        </View>

        {/* Server connection step */}
        {step === 'server' && (
          <View style={styles.stepContent}>
            <Text style={[styles.stepTitle, {color: colors.text}]}>
              Connect to your NithronOS server
            </Text>
            <Text style={[styles.stepDescription, {color: `${colors.text}80`}]}>
              Enter the URL of your NithronOS server to get started
            </Text>

            <View style={[styles.inputContainer, {borderColor: colors.border}]}>
              <Icon name="server" size={20} color={colors.text} style={styles.inputIcon} />
              <TextInput
                style={[styles.input, {color: colors.text}]}
                placeholder="https://your-server.com"
                placeholderTextColor={`${colors.text}50`}
                value={serverUrl}
                onChangeText={setServerUrl}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
                returnKeyType="go"
                onSubmitEditing={handleConnect}
              />
            </View>

            <TouchableOpacity
              style={[
                styles.button,
                {backgroundColor: colors.primary},
                (!serverUrl.trim() || isConnecting) && styles.buttonDisabled,
              ]}
              onPress={handleConnect}
              disabled={!serverUrl.trim() || isConnecting}>
              {isConnecting ? (
                <ActivityIndicator color="#FFFFFF" />
              ) : (
                <>
                  <Text style={styles.buttonText}>Connect</Text>
                  <Icon name="arrow-right" size={20} color="#FFFFFF" />
                </>
              )}
            </TouchableOpacity>

            {/* Help text */}
            <View style={styles.helpSection}>
              <Icon name="help-circle-outline" size={18} color={`${colors.text}60`} />
              <Text style={[styles.helpText, {color: `${colors.text}60`}]}>
                You can find your server URL in the NithronOS dashboard under Settings → Remote Access
              </Text>
            </View>
          </View>
        )}

        {/* Device registration step */}
        {step === 'device' && (
          <View style={styles.stepContent}>
            <View style={[styles.successBadge, {backgroundColor: `${colors.primary}20`}]}>
              <Icon name="check-circle" size={24} color={colors.primary} />
              <Text style={[styles.successText, {color: colors.primary}]}>
                Connected to server
              </Text>
            </View>

            <Text style={[styles.stepTitle, {color: colors.text}]}>
              Name this device
            </Text>
            <Text style={[styles.stepDescription, {color: `${colors.text}80`}]}>
              Give this device a recognizable name to identify it in your device list
            </Text>

            <View style={[styles.inputContainer, {borderColor: colors.border}]}>
              <Icon name="cellphone" size={20} color={colors.text} style={styles.inputIcon} />
              <TextInput
                style={[styles.input, {color: colors.text}]}
                placeholder="My iPhone"
                placeholderTextColor={`${colors.text}50`}
                value={deviceName}
                onChangeText={setDeviceName}
                autoCapitalize="words"
                returnKeyType="done"
                onSubmitEditing={handleRegister}
              />
            </View>

            <TouchableOpacity
              style={[
                styles.button,
                {backgroundColor: colors.primary},
                (!deviceName.trim() || isLoading) && styles.buttonDisabled,
              ]}
              onPress={handleRegister}
              disabled={!deviceName.trim() || isLoading}>
              {isLoading ? (
                <ActivityIndicator color="#FFFFFF" />
              ) : (
                <>
                  <Text style={styles.buttonText}>Complete Setup</Text>
                  <Icon name="check" size={20} color="#FFFFFF" />
                </>
              )}
            </TouchableOpacity>

            {/* Back button */}
            <TouchableOpacity
              style={styles.backButton}
              onPress={() => setStep('server')}>
              <Icon name="arrow-left" size={18} color={colors.primary} />
              <Text style={[styles.backButtonText, {color: colors.primary}]}>
                Use different server
              </Text>
            </TouchableOpacity>
          </View>
        )}

        {/* Error display */}
        {error && (
          <View style={[styles.errorContainer, {backgroundColor: '#FEE2E2'}]}>
            <Icon name="alert-circle" size={20} color="#DC2626" />
            <Text style={styles.errorText}>{error}</Text>
            <TouchableOpacity onPress={clearError}>
              <Icon name="close" size={18} color="#DC2626" />
            </TouchableOpacity>
          </View>
        )}
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

interface StepBadgeProps {
  number: number;
  label: string;
  active: boolean;
  completed: boolean;
  colors: {primary: string; text: string; border: string; background: string};
}

function StepBadge({number, label, active, completed, colors}: StepBadgeProps) {
  const bgColor = completed ? colors.primary : active ? colors.primary : colors.border;
  const textColor = completed || active ? '#FFFFFF' : colors.text;

  return (
    <View style={styles.stepBadgeContainer}>
      <View style={[styles.stepBadge, {backgroundColor: bgColor}]}>
        {completed ? (
          <Icon name="check" size={16} color={textColor} />
        ) : (
          <Text style={[styles.stepNumber, {color: textColor}]}>{number}</Text>
        )}
      </View>
      <Text style={[styles.stepLabel, {color: active ? colors.primary : `${colors.text}80`}]}>
        {label}
      </Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
  },
  content: {
    padding: 24,
  },
  stepIndicator: {
    marginBottom: 32,
  },
  stepRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
  },
  stepBadgeContainer: {
    alignItems: 'center',
  },
  stepBadge: {
    width: 32,
    height: 32,
    borderRadius: 16,
    justifyContent: 'center',
    alignItems: 'center',
    marginBottom: 6,
  },
  stepNumber: {
    fontSize: 14,
    fontWeight: '600',
  },
  stepLabel: {
    fontSize: 12,
    fontWeight: '500',
  },
  stepLine: {
    height: 2,
    width: 80,
    marginHorizontal: 8,
    marginBottom: 20,
  },
  stepContent: {
    flex: 1,
  },
  stepTitle: {
    fontSize: 22,
    fontWeight: '700',
    marginBottom: 8,
  },
  stepDescription: {
    fontSize: 15,
    lineHeight: 22,
    marginBottom: 24,
  },
  inputContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    borderWidth: 1.5,
    borderRadius: 12,
    paddingHorizontal: 14,
    marginBottom: 16,
  },
  inputIcon: {
    marginRight: 10,
  },
  input: {
    flex: 1,
    fontSize: 16,
    paddingVertical: 14,
  },
  button: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    paddingVertical: 16,
    borderRadius: 12,
    gap: 8,
  },
  buttonDisabled: {
    opacity: 0.5,
  },
  buttonText: {
    color: '#FFFFFF',
    fontSize: 17,
    fontWeight: '600',
  },
  helpSection: {
    flexDirection: 'row',
    marginTop: 20,
    paddingHorizontal: 4,
  },
  helpText: {
    flex: 1,
    fontSize: 13,
    lineHeight: 18,
    marginLeft: 8,
  },
  successBadge: {
    flexDirection: 'row',
    alignItems: 'center',
    alignSelf: 'flex-start',
    paddingHorizontal: 12,
    paddingVertical: 8,
    borderRadius: 8,
    marginBottom: 20,
    gap: 6,
  },
  successText: {
    fontSize: 14,
    fontWeight: '500',
  },
  backButton: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    marginTop: 16,
    gap: 6,
  },
  backButtonText: {
    fontSize: 15,
    fontWeight: '500',
  },
  errorContainer: {
    flexDirection: 'row',
    alignItems: 'center',
    padding: 12,
    borderRadius: 8,
    marginTop: 16,
    gap: 8,
  },
  errorText: {
    flex: 1,
    color: '#DC2626',
    fontSize: 14,
  },
});

export default ManualSetupScreen;

