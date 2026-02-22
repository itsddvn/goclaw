import { useState, useEffect, useCallback } from "react";
import { useHttp } from "@/hooks/use-ws";

export interface ChannelInstanceData {
  id: string;
  name: string;
  display_name: string;
  channel_type: string;
  agent_id: string;
  config: Record<string, unknown> | null;
  enabled: boolean;
  is_default: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface ChannelInstanceInput {
  name: string;
  display_name?: string;
  channel_type: string;
  agent_id: string;
  credentials?: string; // plaintext JSON, encrypted server-side
  config?: Record<string, unknown>;
  enabled?: boolean;
}

export function useChannelInstances() {
  const http = useHttp();
  const [instances, setInstances] = useState<ChannelInstanceData[]>([]);
  const [loading, setLoading] = useState(false);
  const [supported, setSupported] = useState(true); // false if standalone mode (404)

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await http.get<{ instances: ChannelInstanceData[] }>("/v1/channels/instances");
      setInstances(res.instances ?? []);
      setSupported(true);
    } catch (err: unknown) {
      // 404 means standalone mode — channel instances not available
      if (err instanceof Error && err.message.includes("404")) {
        setSupported(false);
      }
    } finally {
      setLoading(false);
    }
  }, [http]);

  useEffect(() => {
    load();
  }, [load]);

  const createInstance = useCallback(
    async (data: ChannelInstanceInput) => {
      const res = await http.post<{ id: string }>("/v1/channels/instances", data);
      await load();
      return res;
    },
    [http, load],
  );

  const updateInstance = useCallback(
    async (id: string, data: Partial<ChannelInstanceInput>) => {
      await http.put(`/v1/channels/instances/${id}`, data);
      await load();
    },
    [http, load],
  );

  const deleteInstance = useCallback(
    async (id: string) => {
      await http.delete(`/v1/channels/instances/${id}`);
      await load();
    },
    [http, load],
  );

  return { instances, loading, supported, refresh: load, createInstance, updateInstance, deleteInstance };
}
