import { useEffect, useMemo, useState } from "react";
import { Pencil, Plus, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { cn } from "@/lib/utils";
import { useAgents } from "@/pages/agents/hooks/use-agents";
import type { Workstation, WorkstationAgentGrant } from "./hooks/use-workstations";

interface WorkstationGrantsDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workstation: Workstation;
  onGrant: (agentId: string, isDefault: boolean) => Promise<void>;
  onRevoke: (agentId: string) => Promise<void>;
  onLoadGrants: (workstationId: string) => Promise<WorkstationAgentGrant[]>;
}

export function WorkstationGrantsDialog({
  open,
  onOpenChange,
  workstation,
  onGrant,
  onRevoke,
  onLoadGrants,
}: WorkstationGrantsDialogProps) {
  const { t } = useTranslation("workstations");
  const { agents } = useAgents();
  const [grants, setGrants] = useState<WorkstationAgentGrant[]>([]);
  const [agentId, setAgentId] = useState("");
  const [isDefault, setIsDefault] = useState(false);
  const [editing, setEditing] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setAgentId("");
    setIsDefault(false);
    setEditing(false);
    setError("");
    setLoading(true);
    onLoadGrants(workstation.id)
      .then(setGrants)
      .catch((err: unknown) => {
        setGrants([]);
        setError(err instanceof Error ? err.message : t("grants.failedLoad"));
      })
      .finally(() => setLoading(false));
  }, [open, workstation.id, onLoadGrants, t]);

  const agentNameMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const agent of agents) map.set(agent.id, agent.display_name || agent.agent_key);
    return map;
  }, [agents]);

  const clearForm = () => {
    setAgentId("");
    setIsDefault(false);
    setEditing(false);
    setError("");
  };

  const selectGrant = (grant: WorkstationAgentGrant) => {
    setAgentId(grant.agentId);
    setIsDefault(grant.isDefault);
    setEditing(true);
    setError("");
  };

  const refreshGrants = async () => {
    setGrants(await onLoadGrants(workstation.id));
  };

  const handleGrant = async () => {
    if (!agentId) {
      setError(t("grants.agentRequired"));
      return;
    }
    setLoading(true);
    setError("");
    try {
      await onGrant(agentId, isDefault);
      await refreshGrants();
      clearForm();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("grants.failedGrant"));
    } finally {
      setLoading(false);
    }
  };

  const handleRevoke = async (grant: WorkstationAgentGrant) => {
    setLoading(true);
    setError("");
    try {
      await onRevoke(grant.agentId);
      await refreshGrants();
      if (agentId === grant.agentId) clearForm();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t("grants.failedRevoke"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] flex flex-col sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{t("grants.title", { name: workstation.name })}</DialogTitle>
          <DialogDescription>{t("grants.description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-4 -mx-4 px-4 sm:-mx-6 sm:px-6 overflow-y-auto min-h-0">
          {grants.length > 0 && (
            <div className="space-y-2">
              <Label>{t("grants.currentGrants")}</Label>
              <div className="grid gap-2">
                {grants.map((grant) => (
                  <div
                    key={grant.agentId}
                    className={cn(
                      "flex items-center rounded-md border transition-colors",
                      editing && agentId === grant.agentId
                        ? "border-ring bg-accent/50 ring-1 ring-ring/30"
                        : "bg-muted/30 hover:bg-muted/50",
                    )}
                  >
                    <button
                      type="button"
                      className="min-w-0 flex-1 px-3 py-2.5 text-left"
                      onClick={() => selectGrant(grant)}
                    >
                      <div className="flex min-w-0 items-center gap-2">
                        <span className="truncate text-sm font-medium">
                          {agentNameMap.get(grant.agentId) || grant.agentId}
                        </span>
                        {grant.isDefault && <Badge variant="secondary">{t("grants.defaultBadge")}</Badge>}
                        {editing && agentId === grant.agentId && <Pencil className="h-3 w-3 text-muted-foreground" />}
                      </div>
                    </button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="mr-2 h-7 w-7 shrink-0"
                      onClick={() => void handleRevoke(grant)}
                      disabled={loading}
                      aria-label={t("grants.revoke")}
                    >
                      <Trash2 className="h-3.5 w-3.5 text-destructive" />
                    </Button>
                  </div>
                ))}
              </div>
            </div>
          )}

          <div className="space-y-3 rounded-md border p-3">
            <div className="flex items-center justify-between">
              <Label>{editing ? t("grants.editGrant") : t("grants.addGrant")}</Label>
              {editing && (
                <Button variant="ghost" size="sm" onClick={clearForm} className="h-6 px-2 text-xs">
                  {t("grants.cancel")}
                </Button>
              )}
            </div>
            <Select value={agentId} onValueChange={setAgentId} disabled={editing}>
              <SelectTrigger className="text-base md:text-sm">
                <SelectValue placeholder={t("grants.selectAgent")} />
              </SelectTrigger>
              <SelectContent>
                {agents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>
                    {agent.display_name || agent.agent_key}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="flex items-start justify-between gap-4 rounded-md bg-muted/30 p-3">
              <div>
                <Label htmlFor="workstation-default-grant">{t("grants.defaultLabel")}</Label>
                <p className="mt-0.5 text-xs text-muted-foreground">{t("grants.defaultHint")}</p>
              </div>
              <Switch
                id="workstation-default-grant"
                checked={isDefault}
                onCheckedChange={setIsDefault}
                disabled={loading}
              />
            </div>
            <Button size="sm" onClick={() => void handleGrant()} disabled={loading || !agentId} className="gap-1">
              {editing ? <Pencil className="h-3.5 w-3.5" /> : <Plus className="h-3.5 w-3.5" />}
              {editing ? t("grants.update") : t("grants.grant")}
            </Button>
          </div>

          {loading && grants.length === 0 && <p className="text-sm text-muted-foreground">{t("grants.loading")}</p>}
          {error && <p className="text-sm text-destructive">{error}</p>}
        </div>
      </DialogContent>
    </Dialog>
  );
}
