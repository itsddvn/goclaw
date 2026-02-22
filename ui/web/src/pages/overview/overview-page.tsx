import { useEffect, useState, useCallback } from "react";
import { Activity, Bot, History, Zap } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatusBadge } from "@/components/shared/status-badge";
import { useAuthStore } from "@/stores/use-auth-store";
import { useWsCall } from "@/hooks/use-ws-call";
import { useWsEvent } from "@/hooks/use-ws-event";
import { Methods, Events } from "@/api/protocol";

interface HealthPayload {
  status?: string;
  uptime?: number;
}

interface AgentInfo {
  id: string;
  model: string;
  isRunning: boolean;
}

interface StatusPayload {
  agents?: AgentInfo[];
  sessions?: number;
  clients?: number;
}

function StatCard({
  icon: Icon,
  label,
  value,
}: {
  icon: React.ElementType;
  label: string;
  value: string | number;
}) {
  return (
    <div className="rounded-lg border bg-card p-6">
      <div className="flex items-center gap-3">
        <div className="rounded-md bg-muted p-2">
          <Icon className="h-4 w-4 text-muted-foreground" />
        </div>
        <div>
          <p className="text-sm text-muted-foreground">{label}</p>
          <p className="text-2xl font-semibold">{value}</p>
        </div>
      </div>
    </div>
  );
}

export function OverviewPage() {
  const connected = useAuthStore((s) => s.connected);
  const { call: fetchHealth, data: health } = useWsCall<HealthPayload>(Methods.HEALTH);
  const { call: fetchStatus, data: status } = useWsCall<StatusPayload>(Methods.STATUS);
  const [, setLastUpdate] = useState(0);

  useEffect(() => {
    if (connected) {
      fetchHealth();
      fetchStatus();
    }
  }, [connected, fetchHealth, fetchStatus]);

  // Re-fetch on health events
  const handleHealth = useCallback(() => {
    setLastUpdate(Date.now());
    fetchHealth();
  }, [fetchHealth]);

  useWsEvent(Events.HEALTH, handleHealth);

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Overview"
        description="Gateway status and health"
        actions={
          <StatusBadge
            status={connected ? "success" : "error"}
            label={connected ? "Connected" : "Disconnected"}
          />
        }
      />

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={Activity}
          label="Status"
          value={health?.status ?? "unknown"}
        />
        <StatCard
          icon={Bot}
          label="Agents"
          value={status?.agents?.length ?? 0}
        />
        <StatCard
          icon={History}
          label="Sessions"
          value={status?.sessions ?? 0}
        />
        <StatCard
          icon={Zap}
          label="Connected Clients"
          value={status?.clients ?? 0}
        />
      </div>
    </div>
  );
}
