/**
 * Pairs from a pasted pairing link: the same link the web interface's
 * QR code carries, shown under it as text.
 */

import React, {useState} from 'react';
import {
  ActivityIndicator,
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  TouchableOpacity,
  View,
} from 'react-native';
import {useTheme} from '@react-navigation/native';
import Icon from 'react-native-vector-icons/MaterialCommunityIcons';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {explainBadLink, PAIRING_STEPS} from './pairingHelp';

export function ManualSetupScreen(): React.JSX.Element {
  const {colors} = useTheme();
  const {pairWithLink, isBusy} = useAuthStore();
  const [link, setLink] = useState('');
  const [failure, setFailure] = useState<string | null>(null);

  const canPair = link.trim() !== '' && !isBusy;

  const handlePair = async () => {
    if (!canPair) {
      return;
    }
    setFailure(null);
    try {
      await pairWithLink(link);
      // Paired: the navigator switches to the app.
    } catch (error) {
      setFailure(explainBadLink(link, errorMessage(error)));
    }
  };

  return (
    <KeyboardAvoidingView
      style={styles.flex}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView
        style={[styles.flex, {backgroundColor: colors.background}]}
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled">
        <Text style={[styles.title, {color: colors.text}]}>
          Paste the pairing link
        </Text>
        <Text style={[styles.description, {color: `${colors.text}99`}]}>
          {PAIRING_STEPS}
        </Text>

        <TextInput
          accessibilityLabel="Pairing link"
          style={[
            styles.input,
            {color: colors.text, borderColor: colors.border},
          ]}
          placeholder="nos://sync?server=…"
          placeholderTextColor={`${colors.text}50`}
          value={link}
          onChangeText={setLink}
          autoCapitalize="none"
          autoCorrect={false}
          multiline
          textAlignVertical="top"
        />

        <TouchableOpacity
          accessibilityRole="button"
          accessibilityState={{disabled: !canPair}}
          style={[
            styles.button,
            {backgroundColor: colors.primary},
            !canPair && styles.buttonDisabled,
          ]}
          onPress={handlePair}
          disabled={!canPair}>
          {isBusy ? (
            <ActivityIndicator color="#FFFFFF" />
          ) : (
            <>
              <Text style={styles.buttonText}>Pair</Text>
              <Icon name="link-variant" size={20} color="#FFFFFF" />
            </>
          )}
        </TouchableOpacity>

        {failure && (
          <View style={styles.errorContainer} accessibilityLiveRegion="polite">
            <Icon name="alert-circle" size={20} color="#DC2626" />
            <Text style={styles.errorText}>{failure}</Text>
          </View>
        )}

        <View style={styles.helpSection}>
          <Icon
            name="shield-lock-outline"
            size={18}
            color={`${colors.text}80`}
          />
          <Text style={[styles.helpText, {color: `${colors.text}80`}]}>
            The link carries this device's sign-in and your server's key, so
            NithronSync trusts that server's own certificate. Keep the link to
            yourself.
          </Text>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  flex: {
    flex: 1,
  },
  content: {
    padding: 24,
  },
  title: {
    fontSize: 22,
    fontWeight: '700',
    marginBottom: 8,
  },
  description: {
    fontSize: 15,
    lineHeight: 22,
    marginBottom: 24,
  },
  input: {
    borderWidth: 1.5,
    borderRadius: 12,
    padding: 14,
    fontSize: 15,
    minHeight: 120,
    marginBottom: 16,
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
  errorContainer: {
    flexDirection: 'row',
    alignItems: 'flex-start',
    padding: 12,
    borderRadius: 8,
    marginTop: 16,
    gap: 8,
    backgroundColor: '#FEE2E2',
  },
  errorText: {
    flex: 1,
    color: '#991B1B',
    fontSize: 14,
    lineHeight: 20,
  },
  helpSection: {
    flexDirection: 'row',
    marginTop: 24,
    gap: 8,
  },
  helpText: {
    flex: 1,
    fontSize: 13,
    lineHeight: 18,
  },
});

export default ManualSetupScreen;
