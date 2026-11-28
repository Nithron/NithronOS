/**
 * Pairs from a pasted pairing link: the same link the web interface's
 * QR code carries, shown under it as text.
 */

import React, {useState} from 'react';
import {
  KeyboardAvoidingView,
  Platform,
  ScrollView,
  StyleSheet,
  Text,
} from 'react-native';

import {errorMessage, useAuthStore} from '../../stores/authStore';
import {Button} from '../../components/Button';
import {Field} from '../../components/Field';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';
import {explainBadLink, PAIRING_STEPS} from './pairingHelp';

export function ManualSetupScreen(): React.JSX.Element {
  const styles = useThemedStyles(setupStyles);
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
      style={layout.flex}
      behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
      <ScrollView
        style={styles.screen}
        contentContainerStyle={styles.content}
        keyboardShouldPersistTaps="handled">
        <Text style={styles.description}>{PAIRING_STEPS}</Text>

        <Field
          label="Pairing link"
          placeholder="nos://sync?server=…"
          value={link}
          onChangeText={setLink}
          autoCapitalize="none"
          autoCorrect={false}
          error={failure}
          multiline
          mono
        />

        <Button
          kind="primary"
          label={isBusy ? 'Pairing' : 'Pair'}
          onPress={handlePair}
          disabled={!canPair}
          block
        />

        <Text style={styles.help}>
          The link carries this device's sign-in and your server's key, so
          NithronSync trusts that server's own certificate. Keep the link to
          yourself.
        </Text>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const layout = StyleSheet.create({flex: {flex: 1}});

const setupStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {padding: metrics.gutter, gap: metrics.space5},
  description: {...type.small, color: p.ink},
  help: {...type.fine, color: p.ink2},
});

export default ManualSetupScreen;
