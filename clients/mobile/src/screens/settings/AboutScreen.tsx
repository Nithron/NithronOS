/**
 * About Screen
 * The app's name and version, what it does, and where to read more.
 */

import React, {useState} from 'react';
import {Linking, ScrollView, Text, View} from 'react-native';

import {Group, Row} from '../../components/layout';
import {PixelText, StatusMark} from '../../components/pixel';
import {APP_VERSION} from '../../version';
import {metrics, Palette, type, useThemedStyles} from '../../theme/theme';

interface Link {
  title: string;
  detail: string;
  url: string;
}

const READ_MORE: Link[] = [
  {title: 'Nithron', detail: 'nithron.com', url: 'https://nithron.com'},
  {
    title: 'Documentation',
    detail: 'nos.nithron.com/docs/sync',
    url: 'https://nos.nithron.com/docs/sync/',
  },
  {
    title: 'Community',
    detail: 'discord.gg/qzB37WS5AT',
    url: 'https://discord.gg/qzB37WS5AT',
  },
  {
    title: 'Email',
    detail: 'hello@nithron.com',
    url: 'mailto:hello@nithron.com',
  },
];

const LEGAL: Link[] = [
  {
    title: 'Privacy policy',
    detail: 'nithron.com/privacy',
    url: 'https://nithron.com/privacy',
  },
  {
    title: 'Terms of service',
    detail: 'nithron.com/terms',
    url: 'https://nithron.com/terms',
  },
  {
    title: 'Licences',
    detail: 'nithron.com/licenses',
    url: 'https://nithron.com/licenses',
  },
];

export function AboutScreen(): React.JSX.Element {
  const styles = useThemedStyles(aboutStyles);
  const [failedLink, setFailedLink] = useState<string | null>(null);

  const open = (url: string) => {
    setFailedLink(null);
    Linking.openURL(url).catch(() => setFailedLink(url));
  };

  const rows = (links: Link[]) =>
    links.map(link => (
      <Row
        key={link.url}
        title={link.title}
        detail={link.detail}
        onPress={() => open(link.url)}
        external
      />
    ));

  return (
    <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
      <View style={styles.name}>
        <PixelText size={24} accessibilityRole="header">
          NithronSync
        </PixelText>
        <Text style={styles.version} selectable>
          Version {APP_VERSION}
        </Text>
        <Text style={styles.description}>
          NithronSync browses the shares of a paired nOS machine. It does not
          sync files to this phone.
        </Text>
      </View>

      {failedLink ? (
        <View style={styles.problem} accessibilityLiveRegion="polite">
          <StatusMark state="failed">{`Couldn't open ${failedLink}`}</StatusMark>
        </View>
      ) : null}

      <Group label="Read more">{rows(READ_MORE)}</Group>
      <Group label="Legal">{rows(LEGAL)}</Group>

      <Text style={styles.footer}>
        © 2025-2026 Thiago Benkendorf &amp; Nithron. nOS and NithronSync are
        source-available under the Nithron Community License (NCL) v1.0.
      </Text>
    </ScrollView>
  );
}

const aboutStyles = (p: Palette) => ({
  screen: {flex: 1, backgroundColor: p.page},
  content: {paddingBottom: metrics.space6},
  name: {
    paddingHorizontal: metrics.gutter,
    paddingTop: metrics.space5,
    gap: metrics.space2,
  },
  version: {...type.mono, color: p.ink2},
  description: {...type.small, color: p.ink, marginTop: metrics.space2},
  problem: {paddingHorizontal: metrics.gutter, paddingTop: metrics.space4},
  footer: {
    ...type.fine,
    color: p.ink2,
    paddingHorizontal: metrics.gutter,
    marginTop: metrics.space6,
  },
});

export default AboutScreen;
