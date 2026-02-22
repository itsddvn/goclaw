import { useState, useEffect, useCallback } from "react";
import { useHttp } from "@/hooks/use-ws";

export interface MCPServerData {
  id: string;
  name: string;
  display_name: string;
  transport: "stdio" | "sse" | "streamable-http";
  command: string;
  args: string[] | null;
  url: string;
  headers: Record<string, string> | null;
  env: Record<string, string> | null;
  tool_prefix: string;
  timeout_sec: number;
  enabled: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface MCPServerInput {
  name: string;
  display_name?: string;
  transport: string;
  command?: string;
  args?: string[];
  url?: string;
  headers?: Record<string, string>;
  tool_prefix?: string;
  timeout_sec?: number;
  enabled?: boolean;
}

export interface MCPAgentGrant {
  id: string;
  server_id: string;
  agent_id: string;
  enabled: boolean;
  tool_allow: string[] | null;
  tool_deny: string[] | null;
  granted_by: string;
  created_at: string;
}

export function useMCP() {
  const http = useHttp();
  const [servers, setServers] = useState<MCPServerData[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await http.get<{ servers: MCPServerData[] }>("/v1/mcp/servers");
      setServers(res.servers ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [http]);

  useEffect(() => {
    load();
  }, [load]);

  const createServer = useCallback(
    async (data: MCPServerInput) => {
      const res = await http.post<MCPServerData>("/v1/mcp/servers", data);
      await load();
      return res;
    },
    [http, load],
  );

  const updateServer = useCallback(
    async (id: string, data: Partial<MCPServerInput>) => {
      await http.put(`/v1/mcp/servers/${id}`, data);
      await load();
    },
    [http, load],
  );

  const deleteServer = useCallback(
    async (id: string) => {
      await http.delete(`/v1/mcp/servers/${id}`);
      await load();
    },
    [http, load],
  );

  const listAgentGrants = useCallback(
    async (_serverId: string) => {
      // Backend provides per-agent listing, not per-server.
      // Grants dialog handles this by accepting agent IDs.
      return [] as MCPAgentGrant[];
    },
    [],
  );

  const grantAgent = useCallback(
    async (serverId: string, agentId: string, toolAllow?: string[], toolDeny?: string[]) => {
      await http.post(`/v1/mcp/servers/${serverId}/grants/agent`, {
        agent_id: agentId,
        tool_allow: toolAllow,
        tool_deny: toolDeny,
      });
    },
    [http],
  );

  const revokeAgent = useCallback(
    async (serverId: string, agentId: string) => {
      await http.delete(`/v1/mcp/servers/${serverId}/grants/agent/${agentId}`);
    },
    [http],
  );

  const listGrantsByAgent = useCallback(
    async (agentId: string) => {
      const res = await http.get<{ grants: MCPAgentGrant[] }>(`/v1/mcp/grants/agent/${agentId}`);
      return res.grants ?? [];
    },
    [http],
  );

  return {
    servers,
    loading,
    refresh: load,
    createServer,
    updateServer,
    deleteServer,
    listAgentGrants,
    grantAgent,
    revokeAgent,
    listGrantsByAgent,
  };
}
