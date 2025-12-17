/**
 * Real-time collaboration client for NithronSync
 */

// Message types
export type MessageType =
  | 'connect'
  | 'disconnect'
  | 'ping'
  | 'pong'
  | 'presence.join'
  | 'presence.leave'
  | 'presence.update'
  | 'presence.list'
  | 'file.change'
  | 'file.create'
  | 'file.delete'
  | 'file.rename'
  | 'file.lock'
  | 'file.unlock'
  | 'cursor.move'
  | 'cursor.select'
  | 'edit.start'
  | 'edit.op'
  | 'edit.end'
  | 'edit.sync'
  | 'subscribe'
  | 'unsubscribe'
  | 'error';

export interface RealtimeMessage {
  id: string;
  type: MessageType;
  channel?: string;
  user_id?: string;
  device_id?: string;
  timestamp: string;
  payload?: unknown;
}

export interface PresenceInfo {
  user_id: string;
  username: string;
  device_id: string;
  device_name: string;
  status: 'online' | 'away' | 'busy' | 'offline';
  current_file?: string;
  current_share?: string;
  cursor_pos?: CursorPosition;
  color: string;
  joined_at: string;
  last_activity: string;
}

export interface CursorPosition {
  line: number;
  column: number;
  end_line?: number;
  end_column?: number;
}

export interface FileLock {
  file_id: string;
  share_id: string;
  path: string;
  user_id: string;
  username: string;
  device_id: string;
  lock_type: 'exclusive' | 'shared';
  locked_at: string;
  expires_at: string;
}

export type ConnectionState = 'connecting' | 'connected' | 'disconnected' | 'reconnecting';

export interface RealtimeClientConfig {
  url: string;
  token: string;
  reconnectInterval?: number;
  maxReconnectAttempts?: number;
  pingInterval?: number;
}

export type MessageHandler = (message: RealtimeMessage) => void;
export type PresenceHandler = (presence: PresenceInfo[]) => void;
export type ConnectionHandler = (state: ConnectionState) => void;

/**
 * Real-time collaboration client
 */
export class RealtimeClient {
  private ws: WebSocket | null = null;
  private config: RealtimeClientConfig;
  private state: ConnectionState = 'disconnected';
  private reconnectAttempts = 0;
  private reconnectTimer: NodeJS.Timeout | null = null;
  private pingTimer: NodeJS.Timeout | null = null;
  private subscriptions: Set<string> = new Set();
  
  // Event handlers
  private messageHandlers: Map<MessageType, Set<MessageHandler>> = new Map();
  private channelHandlers: Map<string, Set<MessageHandler>> = new Map();
  private presenceHandlers: Map<string, PresenceHandler> = new Map();
  private connectionHandlers: Set<ConnectionHandler> = new Set();
  
  // Client info (set after connection)
  public clientId: string = '';
  public userId: string = '';
  public deviceId: string = '';
  public color: string = '';

  constructor(config: RealtimeClientConfig) {
    this.config = {
      reconnectInterval: 5000,
      maxReconnectAttempts: 10,
      pingInterval: 30000,
      ...config,
    };
  }

  /**
   * Connect to the WebSocket server
   */
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      if (this.ws?.readyState === WebSocket.OPEN) {
        resolve();
        return;
      }

      this.setState('connecting');

      const url = new URL(this.config.url);
      url.searchParams.set('token', this.config.token);
      
      this.ws = new WebSocket(url.toString());

      this.ws.onopen = () => {
        this.reconnectAttempts = 0;
        this.startPingInterval();
        // Wait for connect message before resolving
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data);
        
        // Resolve promise on connection confirmation
        const message = JSON.parse(event.data) as RealtimeMessage;
        if (message.type === 'connect') {
          this.setState('connected');
          const payload = message.payload as {
            client_id: string;
            user_id: string;
            device_id: string;
            color: string;
          };
          this.clientId = payload.client_id;
          this.userId = payload.user_id;
          this.deviceId = payload.device_id;
          this.color = payload.color;
          
          // Resubscribe to channels
          this.subscriptions.forEach(channel => {
            this.send({ type: 'subscribe', payload: { channel } });
          });
          
          resolve();
        }
      };

      this.ws.onclose = () => {
        this.stopPingInterval();
        this.setState('disconnected');
        this.attemptReconnect();
      };

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error);
        reject(error);
      };
    });
  }

  /**
   * Disconnect from the WebSocket server
   */
  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.stopPingInterval();
    
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    
    this.setState('disconnected');
    this.subscriptions.clear();
  }

  /**
   * Subscribe to a channel
   */
  subscribe(channel: string, handler?: MessageHandler): void {
    this.subscriptions.add(channel);
    
    if (handler) {
      if (!this.channelHandlers.has(channel)) {
        this.channelHandlers.set(channel, new Set());
      }
      this.channelHandlers.get(channel)!.add(handler);
    }
    
    if (this.state === 'connected') {
      this.send({ type: 'subscribe', payload: { channel } });
    }
  }

  /**
   * Unsubscribe from a channel
   */
  unsubscribe(channel: string): void {
    this.subscriptions.delete(channel);
    this.channelHandlers.delete(channel);
    
    if (this.state === 'connected') {
      this.send({ type: 'unsubscribe', payload: { channel } });
    }
  }

  /**
   * Send a message
   */
  send(message: Partial<RealtimeMessage>): void {
    if (this.ws?.readyState !== WebSocket.OPEN) {
      console.warn('WebSocket not connected, message not sent');
      return;
    }

    const fullMessage: RealtimeMessage = {
      id: crypto.randomUUID(),
      timestamp: new Date().toISOString(),
      ...message,
    } as RealtimeMessage;

    this.ws.send(JSON.stringify(fullMessage));
  }

  /**
   * Update presence information
   */
  updatePresence(update: Partial<PresenceInfo>): void {
    this.send({
      type: 'presence.update',
      payload: update,
    });
  }

  /**
   * Get presence list for a channel
   */
  getPresence(channel: string): void {
    this.send({
      type: 'presence.list',
      payload: { channel },
    });
  }

  /**
   * Send cursor position update
   */
  sendCursorMove(channel: string, position: CursorPosition): void {
    this.send({
      type: 'cursor.move',
      payload: { channel, position },
    });
  }

  /**
   * Send selection update
   */
  sendSelection(channel: string, selection: CursorPosition): void {
    this.send({
      type: 'cursor.select',
      payload: { channel, selection },
    });
  }

  /**
   * Request a file lock
   */
  lockFile(shareId: string, path: string, lockType: 'exclusive' | 'shared' = 'exclusive', duration = 300): void {
    this.send({
      type: 'file.lock',
      payload: { share_id: shareId, path, lock_type: lockType, duration },
    });
  }

  /**
   * Release a file lock
   */
  unlockFile(shareId: string, path: string): void {
    this.send({
      type: 'file.unlock',
      payload: { share_id: shareId, path },
    });
  }

  /**
   * Start editing a file
   */
  startEditing(channel: string, fileId: string): void {
    this.send({
      type: 'edit.start',
      payload: { channel, file_id: fileId },
    });
  }

  /**
   * Send an edit operation
   */
  sendEditOp(channel: string, operation: unknown): void {
    this.send({
      type: 'edit.op',
      channel,
      payload: operation,
    });
  }

  /**
   * End editing a file
   */
  endEditing(channel: string, fileId: string): void {
    this.send({
      type: 'edit.end',
      payload: { channel, file_id: fileId },
    });
  }

  /**
   * Register a message handler
   */
  on(type: MessageType, handler: MessageHandler): () => void {
    if (!this.messageHandlers.has(type)) {
      this.messageHandlers.set(type, new Set());
    }
    this.messageHandlers.get(type)!.add(handler);
    
    return () => {
      this.messageHandlers.get(type)?.delete(handler);
    };
  }

  /**
   * Register a presence handler for a channel
   */
  onPresence(channel: string, handler: PresenceHandler): () => void {
    this.presenceHandlers.set(channel, handler);
    return () => {
      this.presenceHandlers.delete(channel);
    };
  }

  /**
   * Register a connection state handler
   */
  onConnectionChange(handler: ConnectionHandler): () => void {
    this.connectionHandlers.add(handler);
    return () => {
      this.connectionHandlers.delete(handler);
    };
  }

  /**
   * Get the current connection state
   */
  getState(): ConnectionState {
    return this.state;
  }

  private handleMessage(data: string): void {
    try {
      const message = JSON.parse(data) as RealtimeMessage;

      // Handle pong
      if (message.type === 'pong') {
        return;
      }

      // Call type-specific handlers
      const typeHandlers = this.messageHandlers.get(message.type);
      if (typeHandlers) {
        typeHandlers.forEach(handler => handler(message));
      }

      // Call channel-specific handlers
      if (message.channel) {
        const channelHandlers = this.channelHandlers.get(message.channel);
        if (channelHandlers) {
          channelHandlers.forEach(handler => handler(message));
        }
      }

      // Handle presence list response
      if (message.type === 'presence.list' && message.channel) {
        const handler = this.presenceHandlers.get(message.channel);
        if (handler) {
          const payload = message.payload as { presence: PresenceInfo[] };
          handler(payload.presence || []);
        }
      }

      // Handle presence updates
      if (message.type === 'presence.join' || message.type === 'presence.leave' || message.type === 'presence.update') {
        // Refresh presence list for affected channels
        if (message.channel) {
          this.getPresence(message.channel);
        }
      }
    } catch (error) {
      console.error('Failed to parse message:', error);
    }
  }

  private setState(state: ConnectionState): void {
    this.state = state;
    this.connectionHandlers.forEach(handler => handler(state));
  }

  private attemptReconnect(): void {
    if (this.reconnectAttempts >= this.config.maxReconnectAttempts!) {
      console.error('Max reconnect attempts reached');
      return;
    }

    this.setState('reconnecting');
    this.reconnectAttempts++;

    const delay = Math.min(
      this.config.reconnectInterval! * Math.pow(2, this.reconnectAttempts - 1),
      30000
    );

    this.reconnectTimer = setTimeout(() => {
      console.log(`Reconnect attempt ${this.reconnectAttempts}...`);
      this.connect().catch(console.error);
    }, delay);
  }

  private startPingInterval(): void {
    this.pingTimer = setInterval(() => {
      this.send({ type: 'ping' });
    }, this.config.pingInterval);
  }

  private stopPingInterval(): void {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
  }
}

// Singleton instance
let realtimeClient: RealtimeClient | null = null;

/**
 * Get or create the realtime client instance
 */
export function getRealtimeClient(config?: RealtimeClientConfig): RealtimeClient {
  if (!realtimeClient && config) {
    realtimeClient = new RealtimeClient(config);
  }
  if (!realtimeClient) {
    throw new Error('Realtime client not initialized');
  }
  return realtimeClient;
}

/**
 * Initialize the realtime client
 */
export function initRealtimeClient(token: string): RealtimeClient {
  const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${wsProtocol}//${window.location.host}/api/v1/sync/ws`;
  
  realtimeClient = new RealtimeClient({ url, token });
  return realtimeClient;
}

