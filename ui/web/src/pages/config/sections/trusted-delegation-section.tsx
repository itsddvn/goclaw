import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Save, ShieldAlert } from "lucide-react";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useHttp } from "@/hooks/use-ws";
import { useAuthStore } from "@/stores/use-auth-store";

const hostGrantSchema = z.object({
  tenant_id: z.string().uuid(),
  agent_id: z.string().uuid(),
});
type HostGrant = z.infer<typeof hostGrantSchema>;
const policySchema = z.object({
  trustedDelegationHostAgents: z.array(hostGrantSchema).optional(),
});
const agentListSchema = z.object({
  agents: z.array(z.object({
    id: z.string().uuid(),
    tenant_id: z.string().uuid().optional(),
    agent_key: z.string(),
    display_name: z.string().nullish(),
  })),
});

interface Props {
  data: unknown;
  onSave: (value: { trustedDelegationHostAgents: HostGrant[] }) => Promise<void>;
  saving: boolean;
}

const grantKey = (grant: HostGrant) => `${grant.tenant_id}:${grant.agent_id}`;

export function TrustedDelegationSection({ data, onSave, saving }: Props) {
  const { t } = useTranslation("config");
  const http = useHttp();
  const connected = useAuthStore((s) => s.connected);
  const tenantId = useAuthStore((s) => s.tenantId);
  const userId = useAuthStore((s) => s.userId);
  const role = useAuthStore((s) => s.role);
  const tenants = useAuthStore((s) => s.availableTenants);
  const policy = useMemo(() => policySchema.safeParse(data ?? {}), [data]);
  const [draft, setDraft] = useState<HostGrant[] | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const canEdit = role === "owner";

  // Use the authoritative HTTP identities, not agents.list's key-only fallback.
  const { data: agentData, isPending, isError, refetch } = useQuery({
    queryKey: ["trusted-delegation-agents", tenantId, userId],
    queryFn: async () => agentListSchema.parse(await http.get<unknown>("/v1/agents")),
    enabled: connected && canEdit,
    staleTime: 60_000,
  });

  useEffect(() => {
    setDraft(null);
    setConfirmOpen(false);
  }, [data]);

  const saved = policy.success ? policy.data.trustedDelegationHostAgents ?? [] : [];
  const grants = draft ?? saved;
  const selected = new Set(grants.map(grantKey));
  const savedKeys = new Set(saved.map(grantKey));
  const dirty = selected.size !== savedKeys.size || [...selected].some((key) => !savedKeys.has(key));
  const rows = new Map<string, { grant: HostGrant; name: string; agentKey?: string }>();
  for (const agent of agentData?.agents ?? []) {
    if (!agent.tenant_id) continue;
    const grant = { tenant_id: agent.tenant_id, agent_id: agent.id };
    rows.set(grantKey(grant), {
      grant,
      name: agent.display_name || agent.agent_key,
      agentKey: agent.agent_key,
    });
  }
  // Keep off-screen/deleted-tenant grants visible and revocable. Never discard
  // another tenant's grant merely because it is absent from the current list.
  for (const grant of [...saved, ...grants]) {
    const key = grantKey(grant);
    if (!rows.has(key)) rows.set(key, { grant, name: grant.agent_id });
  }

  const toggle = (grant: HostGrant, checked: boolean) => {
    const remaining = grants.filter((g) => grantKey(g) !== grantKey(grant));
    setDraft(checked ? [...remaining, grant] : remaining);
  };

  const save = async () => {
    try {
      // Patch only this policy; do not overwrite unrelated tools settings.
      await onSave({ trustedDelegationHostAgents: grants });
      setConfirmOpen(false);
      setDraft(null);
    } catch {
      // useConfig displays the server error and keeps the unsaved draft.
    }
  };

  const requestSave = () => {
    if (grants.some((grant) => !savedKeys.has(grantKey(grant)))) {
      setConfirmOpen(true);
    } else {
      void save();
    }
  };

  return (
    <Card>
      <CardHeader className="gap-3 sm:flex sm:flex-row sm:items-center sm:justify-between">
        <CardTitle className="flex items-center gap-2 text-base">
          <ShieldAlert className="h-4 w-4 shrink-0" />
          {t("trustedDelegation.title")}
        </CardTitle>
        <Button size="sm" className="gap-1.5" onClick={requestSave} disabled={!canEdit || !connected || !policy.success || !dirty || saving}>
          <Save className="h-3.5 w-3.5" />
          {saving ? t("saving") : t("save")}
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        <p className="text-sm text-muted-foreground">{t("trustedDelegation.description")}</p>
        <p className="rounded-md border border-amber-500/30 bg-amber-500/5 p-3 text-sm text-amber-700 dark:text-amber-400">
          {t("trustedDelegation.warning")}
        </p>
        {!canEdit && <p className="text-sm text-muted-foreground">{t("trustedDelegation.ownerOnly")}</p>}
        {!policy.success && <p role="alert" className="text-sm text-destructive">{t("trustedDelegation.invalidConfig")}</p>}
        {canEdit && isPending && <p role="status" className="text-sm text-muted-foreground">{t("trustedDelegation.loading")}</p>}
        {canEdit && isError && (
          <div role="alert" className="flex flex-wrap items-center gap-2 text-sm text-destructive">
            <span>{t("trustedDelegation.loadError")}</span>
            <Button variant="outline" size="sm" onClick={() => void refetch()}>{t("retry")}</Button>
          </div>
        )}
        {!isPending && !isError && rows.size === 0 && <p className="text-sm text-muted-foreground">{t("trustedDelegation.empty")}</p>}
        <div className="space-y-2">
          {[...rows.entries()].map(([key, row]) => {
            const tenant = tenants.find((item) => item.id === row.grant.tenant_id);
            return (
              <div key={key} className="flex items-center justify-between gap-3 rounded-md border p-3">
                <div className="min-w-0 flex-1 space-y-1">
                  <Label htmlFor={`trusted-${key}`} className="break-words text-sm font-medium">{row.name}</Label>
                  {row.agentKey && row.agentKey !== row.name && <p className="break-all font-mono text-xs text-muted-foreground">{row.agentKey}</p>}
                  <p className="break-all text-xs text-muted-foreground">{t("trustedDelegation.tenant", { tenant: tenant?.name || row.grant.tenant_id })}</p>
                </div>
                <Switch id={`trusted-${key}`} checked={selected.has(key)} disabled={!canEdit || !connected || !policy.success || saving || confirmOpen} onCheckedChange={(checked) => toggle(row.grant, checked)} />
              </div>
            );
          })}
        </div>
        <p className="text-xs text-muted-foreground">{t("trustedDelegation.applyHint")}</p>
      </CardContent>
      <ConfirmDialog open={confirmOpen} onOpenChange={setConfirmOpen} title={t("trustedDelegation.confirmTitle")} description={t("trustedDelegation.warning")} confirmLabel={t("trustedDelegation.confirmEnable")} onConfirm={() => void save()} loading={saving} />
    </Card>
  );
}
