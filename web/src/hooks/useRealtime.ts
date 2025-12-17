/**
 * React hooks for real-time collaboration
 */

import { useEffect, useState, useCallback, useRef } from 'react';
import {
  RealtimeClient,
  RealtimeMessage,
  PresenceInfo,
  ConnectionState,
  CursorPosition,
  MessageType,
  initRealtimeClient,
  getRealtimeClient,
} from '@/lib/realtime';

/**
 * Hook to manage the realtime connection
 */
export function useRealtimeConnection(token: string | null) {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected');
  const [client, setClient] = useState<RealtimeClient | null>(null);
  const [error, setError] = useState<Error | null>(null);

  useEffect(() => {
    if (!token) {
      setConnectionState('disconnected');
      return;
    }

    const rtClient = initRealtimeClient(token);
    setClient(rtClient);

    const unsubscribe = rtClient.onConnectionChange(setConnectionState);

    rtClient.connect()
      .then(() => {
        setError(null);
      })
      .catch((err) => {
        setError(err);
      });

    return () => {
      unsubscribe();
      rtClient.disconnect();
    };
  }, [token]);

  return { client, connectionState, error };
}

/**
 * Hook to subscribe to a channel
 */
export function useChannel(channel: string | null) {
  const [messages, setMessages] = useState<RealtimeMessage[]>([]);
  const [presence, setPresence] = useState<PresenceInfo[]>([]);

  useEffect(() => {
    if (!channel) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const handleMessage = (message: RealtimeMessage) => {
      setMessages(prev => [...prev.slice(-99), message]);
    };

    client.subscribe(channel, handleMessage);
    
    const unsubPresence = client.onPresence(channel, setPresence);
    client.getPresence(channel);

    return () => {
      client.unsubscribe(channel);
      unsubPresence();
    };
  }, [channel]);

  const sendMessage = useCallback((type: MessageType, payload: unknown) => {
    if (!channel) return;
    try {
      const client = getRealtimeClient();
      client.send({ type, channel, payload });
    } catch {
      console.error('Failed to send message');
    }
  }, [channel]);

  return { messages, presence, sendMessage };
}

/**
 * Hook for presence information
 */
export function usePresence(channel: string | null) {
  const [presence, setPresence] = useState<PresenceInfo[]>([]);
  const [myPresence, setMyPresence] = useState<Partial<PresenceInfo>>({});

  useEffect(() => {
    if (!channel) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const unsubscribe = client.onPresence(channel, setPresence);
    client.getPresence(channel);

    // Refresh presence periodically
    const interval = setInterval(() => {
      client.getPresence(channel);
    }, 10000);

    return () => {
      unsubscribe();
      clearInterval(interval);
    };
  }, [channel]);

  const updatePresence = useCallback((update: Partial<PresenceInfo>) => {
    try {
      const client = getRealtimeClient();
      client.updatePresence(update);
      setMyPresence(prev => ({ ...prev, ...update }));
    } catch {
      console.error('Failed to update presence');
    }
  }, []);

  return { presence, myPresence, updatePresence };
}

/**
 * Hook for cursor tracking
 */
export function useCursors(channel: string | null) {
  const [cursors, setCursors] = useState<Map<string, { user: PresenceInfo; position: CursorPosition }>>(new Map());

  useEffect(() => {
    if (!channel) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const handleCursorMove = (message: RealtimeMessage) => {
      if (message.type !== 'cursor.move' && message.type !== 'cursor.select') return;
      
      const payload = message.payload as {
        user_id: string;
        username: string;
        color: string;
        position: CursorPosition;
      };

      setCursors(prev => {
        const newMap = new Map(prev);
        newMap.set(payload.user_id, {
          user: {
            user_id: payload.user_id,
            username: payload.username,
            color: payload.color,
          } as PresenceInfo,
          position: payload.position,
        });
        return newMap;
      });
    };

    const unsubMove = client.on('cursor.move', handleCursorMove);
    const unsubSelect = client.on('cursor.select', handleCursorMove);

    // Remove stale cursors
    const cleanup = setInterval(() => {
      setCursors(prev => {
        const newMap = new Map(prev);
        // In a real implementation, we'd track last update time
        return newMap;
      });
    }, 5000);

    return () => {
      unsubMove();
      unsubSelect();
      clearInterval(cleanup);
    };
  }, [channel]);

  const sendCursorPosition = useCallback((position: CursorPosition) => {
    if (!channel) return;
    try {
      const client = getRealtimeClient();
      client.sendCursorMove(channel, position);
    } catch {
      console.error('Failed to send cursor position');
    }
  }, [channel]);

  const sendSelection = useCallback((selection: CursorPosition) => {
    if (!channel) return;
    try {
      const client = getRealtimeClient();
      client.sendSelection(channel, selection);
    } catch {
      console.error('Failed to send selection');
    }
  }, [channel]);

  return { cursors, sendCursorPosition, sendSelection };
}

/**
 * Hook for file locking
 */
export function useFileLock(shareId: string | null, path: string | null) {
  const [isLocked, setIsLocked] = useState(false);
  const [lockedBy, setLockedBy] = useState<{ userId: string; username: string } | null>(null);
  const [isOwner, setIsOwner] = useState(false);

  useEffect(() => {
    if (!shareId || !path) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const channel = `share:${shareId}`;

    const handleLock = (message: RealtimeMessage) => {
      const payload = message.payload as {
        share_id: string;
        path: string;
        user_id: string;
        username: string;
      };

      if (payload.share_id === shareId && payload.path === path) {
        setIsLocked(true);
        setLockedBy({ userId: payload.user_id, username: payload.username });
        setIsOwner(payload.user_id === client.userId);
      }
    };

    const handleUnlock = (message: RealtimeMessage) => {
      const payload = message.payload as { share_id: string; path: string };
      if (payload.share_id === shareId && payload.path === path) {
        setIsLocked(false);
        setLockedBy(null);
        setIsOwner(false);
      }
    };

    const unsubLock = client.on('file.lock', handleLock);
    const unsubUnlock = client.on('file.unlock', handleUnlock);

    client.subscribe(channel);

    return () => {
      unsubLock();
      unsubUnlock();
      client.unsubscribe(channel);
    };
  }, [shareId, path]);

  const lock = useCallback((duration = 300) => {
    if (!shareId || !path) return;
    try {
      const client = getRealtimeClient();
      client.lockFile(shareId, path, 'exclusive', duration);
    } catch {
      console.error('Failed to lock file');
    }
  }, [shareId, path]);

  const unlock = useCallback(() => {
    if (!shareId || !path) return;
    try {
      const client = getRealtimeClient();
      client.unlockFile(shareId, path);
    } catch {
      console.error('Failed to unlock file');
    }
  }, [shareId, path]);

  return { isLocked, lockedBy, isOwner, lock, unlock };
}

/**
 * Hook for collaborative editing
 */
export function useCollaborativeEditing(channel: string | null, fileId: string | null) {
  const [editors, setEditors] = useState<Set<string>>(new Set());
  const [operations, setOperations] = useState<unknown[]>([]);
  const isEditingRef = useRef(false);

  useEffect(() => {
    if (!channel || !fileId) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const handleEditStart = (message: RealtimeMessage) => {
      const payload = message.payload as { user_id: string; file_id: string };
      if (payload.file_id === fileId) {
        setEditors(prev => new Set([...prev, payload.user_id]));
      }
    };

    const handleEditEnd = (message: RealtimeMessage) => {
      const payload = message.payload as { user_id: string; file_id: string };
      if (payload.file_id === fileId) {
        setEditors(prev => {
          const newSet = new Set(prev);
          newSet.delete(payload.user_id);
          return newSet;
        });
      }
    };

    const handleEditOp = (message: RealtimeMessage) => {
      if (message.channel === channel && message.user_id !== client.userId) {
        setOperations(prev => [...prev, message.payload]);
      }
    };

    const unsubStart = client.on('edit.start', handleEditStart);
    const unsubEnd = client.on('edit.end', handleEditEnd);
    const unsubOp = client.on('edit.op', handleEditOp);

    return () => {
      unsubStart();
      unsubEnd();
      unsubOp();
      
      if (isEditingRef.current) {
        client.endEditing(channel, fileId);
      }
    };
  }, [channel, fileId]);

  const startEditing = useCallback(() => {
    if (!channel || !fileId) return;
    try {
      const client = getRealtimeClient();
      client.startEditing(channel, fileId);
      isEditingRef.current = true;
    } catch {
      console.error('Failed to start editing');
    }
  }, [channel, fileId]);

  const endEditing = useCallback(() => {
    if (!channel || !fileId) return;
    try {
      const client = getRealtimeClient();
      client.endEditing(channel, fileId);
      isEditingRef.current = false;
    } catch {
      console.error('Failed to end editing');
    }
  }, [channel, fileId]);

  const sendOperation = useCallback((operation: unknown) => {
    if (!channel) return;
    try {
      const client = getRealtimeClient();
      client.sendEditOp(channel, operation);
    } catch {
      console.error('Failed to send operation');
    }
  }, [channel]);

  const clearOperations = useCallback(() => {
    setOperations([]);
  }, []);

  return {
    editors,
    operations,
    isEditing: isEditingRef.current,
    startEditing,
    endEditing,
    sendOperation,
    clearOperations,
  };
}

/**
 * Hook for file change notifications
 */
export function useFileChanges(shareId: string | null) {
  const [changes, setChanges] = useState<RealtimeMessage[]>([]);

  useEffect(() => {
    if (!shareId) return;

    let client: RealtimeClient;
    try {
      client = getRealtimeClient();
    } catch {
      return;
    }

    const channel = `share:${shareId}`;

    const handleChange = (message: RealtimeMessage) => {
      if (
        message.type === 'file.change' ||
        message.type === 'file.create' ||
        message.type === 'file.delete' ||
        message.type === 'file.rename'
      ) {
        setChanges(prev => [...prev.slice(-49), message]);
      }
    };

    client.subscribe(channel, handleChange);

    return () => {
      client.unsubscribe(channel);
    };
  }, [shareId]);

  const clearChanges = useCallback(() => {
    setChanges([]);
  }, []);

  return { changes, clearChanges };
}

