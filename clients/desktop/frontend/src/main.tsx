import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './styles/nithron-tokens.css';
import './index.css';
import { startPixelSnap } from './pixel-snap';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
startPixelSnap();
