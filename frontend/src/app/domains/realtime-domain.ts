import { useEffect, useRef, useState } from "react";
import type { UserInfo } from "../../types";
import type { WsStatus } from "../types";

export type UseRealtimeConnectionDeps = {
  user: UserInfo | null;
  wsReconnectDelays: number[];
  onMessage: (raw: string) => void;
  onRecover: () => Promise<void>;
};

export function useRealtimeConnection(deps: UseRealtimeConnectionDeps) {
  const { user, wsReconnectDelays, onMessage, onRecover } = deps;
  const [wsStatus, setWsStatus] = useState<WsStatus>("closed");

  const socketRef = useRef<WebSocket | null>(null);
  const reconnectAttemptRef = useRef(0);
  const reconnectTimerRef = useRef(0);
  const connectNowRef = useRef<(() => void) | null>(null);
  const onMessageRef = useRef(onMessage);
  const onRecoverRef = useRef(onRecover);

  useEffect(() => {
    onMessageRef.current = onMessage;
  }, [onMessage]);

  useEffect(() => {
    onRecoverRef.current = onRecover;
  }, [onRecover]);

  useEffect(() => {
    if (!user) {
      window.clearTimeout(reconnectTimerRef.current);
      connectNowRef.current = null;
      reconnectAttemptRef.current = 0;
      socketRef.current?.close();
      socketRef.current = null;
      setWsStatus("closed");
      return;
    }

    let disposed = false;

    const clearReconnectTimer = () => {
      window.clearTimeout(reconnectTimerRef.current);
      reconnectTimerRef.current = 0;
    };

    const scheduleReconnect = () => {
      if (disposed) return;
      clearReconnectTimer();
      const index = Math.min(reconnectAttemptRef.current, wsReconnectDelays.length - 1);
      const delay = wsReconnectDelays[index];
      reconnectAttemptRef.current += 1;
      reconnectTimerRef.current = window.setTimeout(connect, delay);
    };

    const connect = () => {
      if (disposed) return;
      clearReconnectTimer();

      const currentSocket = socketRef.current;
      if (currentSocket && (currentSocket.readyState === WebSocket.OPEN || currentSocket.readyState === WebSocket.CONNECTING)) {
        return;
      }

      setWsStatus("connecting");
      const protocol = window.location.protocol === "https:" ? "wss" : "ws";
      const endpoint = import.meta.env.VITE_WS_URL || `${protocol}://${window.location.host}/ws/chat`;
      const socket = new WebSocket(endpoint);
      socketRef.current = socket;

      socket.onopen = () => {
        if (disposed) return;
        reconnectAttemptRef.current = 0;
        setWsStatus("open");
        void onRecoverRef.current();
      };
      socket.onclose = () => {
        if (disposed) return;
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        setWsStatus("closed");
        scheduleReconnect();
      };
      socket.onerror = () => {
        if (disposed) return;
        setWsStatus("closed");
        socket.close();
      };
      socket.onmessage = (event) => {
        onMessageRef.current(event.data);
      };
    };

    connectNowRef.current = connect;
    connect();

    return () => {
      disposed = true;
      clearReconnectTimer();
      connectNowRef.current = null;
      reconnectAttemptRef.current = 0;
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [user, wsReconnectDelays]);

  useEffect(() => {
    if (!user) return;
    const handleVisible = () => {
      if (document.visibilityState !== "visible") return;
      const socket = socketRef.current;
      if (!socket || socket.readyState === WebSocket.CLOSED || socket.readyState === WebSocket.CLOSING) {
        connectNowRef.current?.();
      }
      void onRecoverRef.current();
    };

    document.addEventListener("visibilitychange", handleVisible);
    return () => {
      document.removeEventListener("visibilitychange", handleVisible);
    };
  }, [user]);

  return {
    wsStatus,
    socketRef
  };
}
