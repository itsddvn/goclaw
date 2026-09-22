import { useCallback, useEffect, useState } from "react";
import { Plus, RefreshCw, ShieldCheck, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Switch } from "@/components/ui/switch";
import type { WorkstationPermission } from "./hooks/use-workstations";
import { isValidCommandPattern } from "./workstation-management-helpers";

interface WorkstationCommandPermissionsTabProps {
  workstationId: string;
  onList: (workstationId: string) => Promise<WorkstationPermission[]>;
  onAdd: (workstationId: string, pattern: string) => Promise<void>;
  onRemove: (workstationId: string, permissionId: string) => Promise<void>;
  onToggle: (workstationId: string, permissionId: string, enabled: boolean) => Promise<void>;
}

export function WorkstationCommandPermissionsTab({
  workstationId,
  onList,
  onAdd,
  onRemove,
  onToggle,
}: WorkstationCommandPermissionsTabProps) {
  const { t } = useTranslation("workstations");
  const [permissions, setPermissions] = useState<WorkstationPermission[]>([]);
  const [pattern, setPattern] = useState("");
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setPermissions(await onList(workstationId));
    } catch (err) {
      setPermissions([]);
      setError(err instanceof Error ? err.message : t("permissions.failedLoad"));
    } finally {
      setLoading(false);
    }
  }, [onList, workstationId, t]);

  useEffect(() => { void load(); }, [load]);

  async function runMutation(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("permissions.failedSave"));
    } finally {
      setBusy(false);
    }
  }

  function handleAdd() {
    const normalized = pattern.trim();
    if (!isValidCommandPattern(normalized)) {
      setError(t("permissions.invalidPattern"));
      return;
    }
    void runMutation(async () => {
      await onAdd(workstationId, normalized);
      setPattern("");
    });
  }

  return (
    <div className="space-y-4 p-1 sm:p-4">
      <Alert>
        <ShieldCheck />
        <AlertTitle>{t("permissions.policyTitle")}</AlertTitle>
        <AlertDescription>{t("permissions.policyDescription")}</AlertDescription>
      </Alert>

      <div className="flex flex-col gap-2 sm:flex-row">
        <Input
          value={pattern}
          onChange={(event) => setPattern(event.target.value)}
          onKeyDown={(event) => { if (event.key === "Enter") handleAdd(); }}
          placeholder={t("permissions.patternPlaceholder")}
          className="font-mono text-base md:text-sm"
          disabled={busy}
          aria-label={t("permissions.patternLabel")}
        />
        <Button size="sm" onClick={handleAdd} disabled={busy || !pattern.trim()} className="gap-1">
          <Plus className="h-3.5 w-3.5" />
          {t("permissions.add")}
        </Button>
      </div>
      <p className="text-xs text-muted-foreground">{t("permissions.patternHint")}</p>

      {loading ? (
        <p className="text-sm text-muted-foreground">{t("permissions.loading")}</p>
      ) : permissions.length === 0 ? (
        <div className="rounded-md border border-dashed p-6 text-center">
          <p className="text-sm font-medium">{t("permissions.emptyTitle")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t("permissions.emptyDescription")}</p>
        </div>
      ) : (
        <div className="divide-y rounded-md border">
          {permissions.map((permission) => (
            <div key={permission.id} className="flex items-center gap-3 px-3 py-2.5">
              <code className="min-w-0 flex-1 truncate text-sm">{permission.pattern}</code>
              <Switch
                checked={permission.enabled}
                onCheckedChange={(enabled) => void runMutation(() => onToggle(workstationId, permission.id, enabled))}
                disabled={busy}
                aria-label={t("permissions.toggle", { pattern: permission.pattern })}
              />
              <Button
                variant="ghost"
                size="icon-sm"
                onClick={() => void runMutation(() => onRemove(workstationId, permission.id))}
                disabled={busy}
                aria-label={t("permissions.remove", { pattern: permission.pattern })}
              >
                <Trash2 className="h-3.5 w-3.5 text-destructive" />
              </Button>
            </div>
          ))}
        </div>
      )}

      <div className="flex items-center justify-between gap-3">
        {error ? <p className="text-sm text-destructive">{error}</p> : <span />}
        <Button variant="ghost" size="sm" onClick={() => void load()} disabled={loading || busy} className="gap-1">
          <RefreshCw className={`h-3.5 w-3.5 ${loading ? "animate-spin" : ""}`} />
          {t("common:refresh", "Refresh")}
        </Button>
      </div>
    </div>
  );
}
