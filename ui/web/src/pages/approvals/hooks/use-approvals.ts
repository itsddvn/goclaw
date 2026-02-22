import { useState, useEffect, useCallback } from "react";
import { useWs } from "@/hooks/use-ws";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Methods, Events } from "@/api/protocol";

export interface PendingApproval {
  id: string;
  command: string;
  agentId: string;
  createdAt: number;
}

export function useApprovals() {
  const ws = useWs();
  const [pending, setPending] = useState<PendingApproval[]>([]);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!ws.isConnected) return;
    setLoading(true);
    try {
      const res = await ws.call<{ pending: PendingApproval[] }>(Methods.APPROVALS_LIST);
      setPending(res.pending ?? []);
    } catch {
      // ignore
    } finally {
      setLoading(false);
    }
  }, [ws]);

  useEffect(() => {
    load();
  }, [load]);

  // Listen for new approval requests
  useWsEvent(Events.EXEC_APPROVAL_REQUESTED, () => {
    load();
  });

  // Listen for resolved approvals
  useWsEvent(Events.EXEC_APPROVAL_RESOLVED, () => {
    load();
  });

  const approve = useCallback(
    async (id: string, always = false) => {
      await ws.call(Methods.APPROVALS_APPROVE, { id, always });
      setPending((prev) => prev.filter((a) => a.id !== id));
    },
    [ws],
  );

  const deny = useCallback(
    async (id: string) => {
      await ws.call(Methods.APPROVALS_DENY, { id });
      setPending((prev) => prev.filter((a) => a.id !== id));
    },
    [ws],
  );

  return { pending, loading, refresh: load, approve, deny };
}
