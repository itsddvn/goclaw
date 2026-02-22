import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { useWsEvent } from "@/hooks/use-ws-event";
import { useDebouncedCallback } from "@/hooks/use-debounced-callback";
import { Methods, Events } from "@/api/protocol";
import type { SessionInfo, SessionPreview, Message } from "@/types/session";
import type { AgentEventPayload } from "@/types/chat";

export function useSessions(agentFilter?: string) {
  const ws = useWs();
  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ sessions: SessionInfo[] }>(Methods.SESSIONS_LIST, {
        agentId: agentFilter || undefined,
      });
      const sorted = (res.sessions ?? []).sort(
        (a: SessionInfo, b: SessionInfo) =>
          new Date(b.updated).getTime() - new Date(a.updated).getTime(),
      );
      setSessions(sorted);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws, agentFilter]);

  useEffect(() => {
    load();
  }, [load]);

  // Auto-refresh when any agent run completes (from any channel)
  const debouncedRefresh = useDebouncedCallback(load, 2000);

  const handleAgentEvent = useCallback(
    (payload: unknown) => {
      const event = payload as AgentEventPayload;
      if (!event) return;
      if (event.type === "run.completed" || event.type === "run.failed") {
        debouncedRefresh();
      }
    },
    [debouncedRefresh],
  );

  useWsEvent(Events.AGENT, handleAgentEvent);

  const preview = useCallback(
    async (key: string): Promise<SessionPreview | null> => {
      if (!ws.isConnected) return null;
      const res = await ws.call<{ key: string; messages: Message[]; summary?: string }>(
        Methods.SESSIONS_PREVIEW,
        { key },
      );
      return { key: res.key, messages: res.messages ?? [], summary: res.summary };
    },
    [ws],
  );

  const deleteSession = useCallback(
    async (key: string) => {
      if (!ws.isConnected) return;
      await ws.call(Methods.SESSIONS_DELETE, { key });
      setSessions((prev) => prev.filter((s) => s.key !== key));
    },
    [ws],
  );

  const resetSession = useCallback(
    async (key: string) => {
      if (!ws.isConnected) return;
      await ws.call(Methods.SESSIONS_RESET, { key });
      load();
    },
    [ws, load],
  );

  const patchSession = useCallback(
    async (key: string, updates: { label?: string; model?: string }) => {
      if (!ws.isConnected) return;
      await ws.call(Methods.SESSIONS_PATCH, { key, ...updates });
      load();
    },
    [ws, load],
  );

  return { sessions, loading, refresh: load, preview, deleteSession, resetSession, patchSession };
}
