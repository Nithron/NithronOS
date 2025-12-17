/**
 * Files Screen (Root)
 * Entry point to file browsing - redirects to SharesScreen
 */

import React from 'react';
import {SharesScreen} from '../files/SharesScreen';

export function FilesScreen(): React.JSX.Element {
  return <SharesScreen />;
}

export default FilesScreen;

