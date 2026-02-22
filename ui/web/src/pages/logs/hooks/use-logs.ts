import { useState, useEffect, useCallback, useRef } from "react";
import { useWs } from "@/hooks/use-ws";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Methods } from "@/api/protocol";

export interface LogEntry {
  timestamp: number;
  level: string;
  message: string;
  source?: string;
}

export function useLogs() {
  const ws = useWs();
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [tailing, setTailing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const maxLogs = useRef(500);

  const startTail = useCallback(async () => {
    if (!ws.isConnected) return;
    setError(null);
    try {
      await ws.call(Methods.LOGS_TAIL, { action: "start" });
      setTailing(true);
    } catch {
      setError("logs.tail is not available on this backend.");
    }
  }, [ws]);

  const stopTail = useCallback(async () => {
    if (!ws.isConnected) return;
    try {
      await ws.call(Methods.LOGS_TAIL, { action: "stop" });
    } catch {
      // ignore
    }
    setTailing(false);
  }, [ws]);

  // Listen for log events
  useWsEvent("log", (payload: unknown) => {
    const entry = payload as LogEntry;
    if (!entry) return;
    setLogs((prev) => {
      const next = [...prev, entry];
      return next.length > maxLogs.current ? next.slice(-maxLogs.current) : next;
    });
  });

  useEffect(() => {
    return () => {
      // Stop tailing on unmount
      if (tailing) {
        stopTail();
      }
    };
  }, [tailing, stopTail]);

  const clearLogs = useCallback(() => {
    setLogs([]);
  }, []);

  return { logs, tailing, error, startTail, stopTail, clearLogs };
}
