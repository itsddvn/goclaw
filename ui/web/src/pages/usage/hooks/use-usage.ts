import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";

export interface UsageRecord {
  agentId: string;
  sessionKey: string;
  model: string;
  provider: string;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
  timestamp: number;
}

export interface UsageSummary {
  byAgent: Record<
    string,
    {
      inputTokens: number;
      outputTokens: number;
      totalTokens: number;
      sessions: number;
    }
  >;
  totalRecords: number;
}

export function useUsage() {
  const ws = useWs();
  const [records, setRecords] = useState<UsageRecord[]>([]);
  const [summary, setSummary] = useState<UsageSummary | null>(null);
  const [loading, setLoading] = useState(false);

  const loadRecords = useCallback(
    async (agentId?: string, limit = 50) => {
      if (!ws.isConnected) return;
      setLoading(true);
      try {
        const res = await ws.call<{ records: UsageRecord[] }>(Methods.USAGE_GET, {
          agentId: agentId || undefined,
          limit,
        });
        setRecords(res.records ?? []);
      } catch {
        // ignore
      } finally {
        setLoading(false);
      }
    },
    [ws],
  );

  const loadSummary = useCallback(async () => {
    if (!ws.isConnected) return;
    try {
      const res = await ws.call<UsageSummary>(Methods.USAGE_SUMMARY);
      setSummary(res);
    } catch {
      // ignore
    }
  }, [ws]);

  useEffect(() => {
    loadRecords();
    loadSummary();
  }, [loadRecords, loadSummary]);

  return { records, summary, loading, loadRecords, loadSummary };
}
