import { useState, useEffect, useCallback } from "react";
import { useWs, useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";
import type { AgentData } from "@/types/agent";

interface AgentInfoWs {
  id: string;
  model: string;
  isRunning: boolean;
}

export function useAgents() {
  const ws = useWs();
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const [agents, setAgents] = useState<AgentData[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      // Try HTTP first (returns full agent data, filtered by user access)
      const res = await http.get<{ agents: AgentData[] }>("/v1/agents");
      if (res.agents && res.agents.length > 0) {
        setAgents(res.agents);
        setLoading(false);
        return;
      }
    } catch {
      // HTTP may fail if user doesn't have access - fall through to WS
    }

    // Fallback: WS agents.list returns all running agents (no access filter)
    try {
      if (!ws.isConnected) {
        setLoading(false);
        return;
      }
      const res = await ws.call<{ agents: AgentInfoWs[] }>(Methods.AGENTS_LIST);
      const wsAgents: AgentData[] = (res.agents ?? []).map((a) => ({
        id: a.id,
        agent_key: a.id,
        owner_id: "",
        provider: "",
        model: a.model,
        context_window: 0,
        max_tool_iterations: 0,
        workspace: "",
        restrict_to_workspace: false,
        agent_type: "open" as const,
        is_default: false,
        status: a.isRunning ? "running" : "idle",
      }));
      setAgents(wsAgents);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load agents");
    } finally {
      setLoading(false);
    }
  }, [http, ws]);

  useEffect(() => {
    if (connected) {
      load();
    }
  }, [connected, load]);

  const createAgent = useCallback(
    async (data: Partial<AgentData>) => {
      const res = await http.post<AgentData>("/v1/agents", data);
      setAgents((prev) => [...prev, res]);
      return res;
    },
    [http],
  );

  const deleteAgent = useCallback(
    async (id: string) => {
      await http.delete(`/v1/agents/${id}`);
      setAgents((prev) => prev.filter((a) => a.id !== id));
    },
    [http],
  );

  return { agents, loading, error, refresh: load, createAgent, deleteAgent };
}
