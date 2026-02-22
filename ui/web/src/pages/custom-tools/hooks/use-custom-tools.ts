import { useState, useEffect, useCallback } from "react";
import { useHttp } from "@/hooks/use-ws";

export interface CustomToolData {
  id: string;
  name: string;
  description: string;
  parameters: Record<string, unknown> | null;
  command: string;
  working_dir: string;
  timeout_seconds: number;
  agent_id: string | null;
  enabled: boolean;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export interface CustomToolInput {
  name: string;
  description?: string;
  parameters?: Record<string, unknown>;
  command: string;
  working_dir?: string;
  timeout_seconds?: number;
  agent_id?: string;
  enabled?: boolean;
}

export function useCustomTools() {
  const http = useHttp();
  const [tools, setTools] = useState<CustomToolData[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await http.get<{ tools: CustomToolData[] }>("/v1/tools/custom");
      setTools(res.tools ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [http]);

  useEffect(() => {
    load();
  }, [load]);

  const createTool = useCallback(
    async (data: CustomToolInput) => {
      const res = await http.post<{ id: string }>("/v1/tools/custom", data);
      await load();
      return res;
    },
    [http, load],
  );

  const updateTool = useCallback(
    async (id: string, data: Partial<CustomToolInput>) => {
      await http.put(`/v1/tools/custom/${id}`, data);
      await load();
    },
    [http, load],
  );

  const deleteTool = useCallback(
    async (id: string) => {
      await http.delete(`/v1/tools/custom/${id}`);
      await load();
    },
    [http, load],
  );

  return { tools, loading, refresh: load, createTool, updateTool, deleteTool };
}
