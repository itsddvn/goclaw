import { useState, useEffect, useCallback } from "react";
import { useHttp, useWs } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";
import { Methods } from "@/api/protocol";

export type WorkstationBackendType = "ssh" | "docker";

export interface Workstation {
  id: string;
  workstationKey: string;
  tenantId?: string;
  name: string;
  backendType: WorkstationBackendType;
  defaultCwd: string;
  active: boolean;
  createdAt: string;
  updatedAt: string;
  createdBy?: string;
  metadataSummary?: WorkstationMetadataSummary;
}

export interface WorkstationMetadataSummary {
  host?: string;
  port?: number;
  user?: string;
  hasKey?: boolean;
  hasPassword?: boolean;
  image?: string;
  containerName?: string;
  socketPath?: string;
}

export interface CreateWorkstationParams {
  workstationKey: string;
  name: string;
  backendType: WorkstationBackendType;
  metadata?: Record<string, unknown>;
}

export interface UpdateWorkstationParams {
  name?: string;
  active?: boolean;
  defaultCwd?: string;
  metadata?: Record<string, unknown>;
}

export interface WorkstationAgentGrant {
  agentId: string;
  workstationId: string;
  tenantId: string;
  isDefault: boolean;
  createdAt: string;
}

export interface WorkstationPermission {
  id: string;
  workstationId: string;
  tenantId: string;
  pattern: string;
  enabled: boolean;
  createdBy: string;
  createdAt: string;
}

export interface WorkstationContactGrant {
  workstationId: string;
  tenantId: string;
  contactId: string;
  channelType: string;
  senderId: string;
  displayName?: string;
  username?: string;
  createdBy: string;
  createdAt: string;
}

export function useWorkstations() {
  const ws = useWs();
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const [workstations, setWorkstations] = useState<Workstation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!connected) return;
    setLoading(true);
    setError(null);
    try {
      const res = await ws.call<{ workstations: Workstation[] }>(Methods.WORKSTATIONS_LIST);
      setWorkstations(res.workstations ?? []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load workstations");
    } finally {
      setLoading(false);
    }
  }, [ws, connected]);

  useEffect(() => {
    load();
  }, [load]);

  const createWorkstation = useCallback(
    async (params: CreateWorkstationParams): Promise<Workstation> => {
      const res = await ws.call<{ workstation: Workstation }>(Methods.WORKSTATIONS_CREATE, params as unknown as Record<string, unknown>);
      await load();
      return res.workstation;
    },
    [ws, load],
  );

  const updateWorkstation = useCallback(
    async (id: string, params: UpdateWorkstationParams): Promise<void> => {
      await http.put(`/v1/workstations/${id}`, params);
      await load();
    },
    [http, load],
  );

  const testWorkstation = useCallback(
    async (id: string): Promise<boolean> => {
      const res = await http.post<{ ok: boolean }>(`/v1/workstations/${id}/test`);
      return res.ok;
    },
    [http],
  );

  const deleteWorkstation = useCallback(
    async (id: string): Promise<void> => {
      await ws.call(Methods.WORKSTATIONS_DELETE, { id });
      await load();
    },
    [ws, load],
  );

  const listAgentGrants = useCallback(
    async (workstationId: string): Promise<WorkstationAgentGrant[]> => {
      const res = await http.get<{ grants: WorkstationAgentGrant[] }>(
        `/v1/workstations/${workstationId}/grants`,
      );
      return res.grants ?? [];
    },
    [http],
  );

  const grantAgent = useCallback(
    async (workstationId: string, agentId: string, isDefault: boolean): Promise<void> => {
      await http.post(`/v1/workstations/${workstationId}/grants/agent`, {
        agentId,
        isDefault,
      });
    },
    [http],
  );

  const revokeAgent = useCallback(
    async (workstationId: string, agentId: string): Promise<void> => {
      await http.delete(`/v1/workstations/${workstationId}/grants/agent/${agentId}`);
    },
    [http],
  );

  const listPermissions = useCallback(
    async (workstationId: string): Promise<WorkstationPermission[]> => {
      const res = await http.get<{ permissions: WorkstationPermission[] }>(
        `/v1/workstations/${workstationId}/permissions`,
      );
      return res.permissions ?? [];
    },
    [http],
  );

  const addPermission = useCallback(
    async (workstationId: string, pattern: string): Promise<void> => {
      await http.post(`/v1/workstations/${workstationId}/permissions`, { pattern });
    },
    [http],
  );

  const removePermission = useCallback(
    async (workstationId: string, permissionId: string): Promise<void> => {
      await http.delete(`/v1/workstations/${workstationId}/permissions/${permissionId}`);
    },
    [http],
  );

  const togglePermission = useCallback(
    async (workstationId: string, permissionId: string, enabled: boolean): Promise<void> => {
      await http.put(`/v1/workstations/${workstationId}/permissions/${permissionId}/toggle`, {
        enabled,
      });
    },
    [http],
  );

  const listContactGrants = useCallback(
    async (workstationId: string): Promise<WorkstationContactGrant[]> => {
      const res = await http.get<{ grants: WorkstationContactGrant[] }>(
        `/v1/workstations/${workstationId}/contact-grants`,
      );
      return res.grants ?? [];
    },
    [http],
  );

  const grantContact = useCallback(
    async (workstationId: string, contactId: string): Promise<void> => {
      await http.post(`/v1/workstations/${workstationId}/contact-grants`, { contactId });
    },
    [http],
  );

  const revokeContact = useCallback(
    async (workstationId: string, contactId: string): Promise<void> => {
      await http.delete(`/v1/workstations/${workstationId}/contact-grants/${contactId}`);
    },
    [http],
  );

  return {
    workstations,
    loading,
    error,
    refresh: load,
    createWorkstation,
    updateWorkstation,
    testWorkstation,
    deleteWorkstation,
    listAgentGrants,
    grantAgent,
    revokeAgent,
    listPermissions,
    addPermission,
    removePermission,
    togglePermission,
    listContactGrants,
    grantContact,
    revokeContact,
  };
}
