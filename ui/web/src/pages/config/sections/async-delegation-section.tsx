import { useEffect, useMemo, useState } from "react";
import { Clock3, Save } from "lucide-react";
import { useTranslation } from "react-i18next";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useAuthStore } from "@/stores/use-auth-store";

const DEFAULT_TIMEOUT_MINUTES = 30;
const MIN_TIMEOUT_MINUTES = 1;
const MAX_TIMEOUT_MINUTES = 60;

const toolsConfigSchema = z.object({
  delegateAsyncTimeoutSeconds: z.number().int().optional(),
});

interface Props {
  data: unknown;
  onSave: (value: { delegateAsyncTimeoutSeconds: number }) => Promise<void>;
  saving: boolean;
}

function effectiveTimeoutMinutes(data: unknown): number {
  const parsed = toolsConfigSchema.safeParse(data ?? {});
  const seconds = parsed.success ? parsed.data.delegateAsyncTimeoutSeconds : undefined;

  if (!seconds) return DEFAULT_TIMEOUT_MINUTES;
  return seconds / 60;
}

export function AsyncDelegationSection({ data, onSave, saving }: Props) {
  const { t } = useTranslation("config");
  const connected = useAuthStore((state) => state.connected);
  const role = useAuthStore((state) => state.role);
  const canEdit = role === "owner";
  const savedMinutes = useMemo(() => effectiveTimeoutMinutes(data), [data]);
  const [draft, setDraft] = useState(() => String(savedMinutes));

  useEffect(() => {
    setDraft(String(savedMinutes));
  }, [savedMinutes]);

  const minutes = Number(draft);
  const invalidMinutes = draft.trim() === "" || !Number.isInteger(minutes) || minutes < MIN_TIMEOUT_MINUTES || minutes > MAX_TIMEOUT_MINUTES;
  const dirty = !invalidMinutes && minutes !== savedMinutes;

  const save = async () => {
    if (invalidMinutes) return;

    try {
      await onSave({ delegateAsyncTimeoutSeconds: minutes * 60 });
    } catch {
      // useConfig displays the server error and keeps the unsaved draft.
    }
  };

  return (
    <Card id="async-delegation" aria-labelledby="async-delegation-title">
      <CardHeader className="gap-2 sm:flex sm:flex-row sm:items-center sm:justify-between">
        <div className="space-y-1">
          <CardTitle id="async-delegation-title" className="flex items-center gap-2 text-base">
            <Clock3 className="h-4 w-4 shrink-0" />
            {t("asyncDelegation.title")}
          </CardTitle>
          <CardDescription>{t("asyncDelegation.description")}</CardDescription>
        </div>
        <Button size="sm" className="gap-1.5" onClick={() => void save()} disabled={!canEdit || !connected || !dirty || saving}>
          <Save className="h-3.5 w-3.5" />
          {saving ? t("saving") : t("save")}
        </Button>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid max-w-sm gap-1.5">
          <Label htmlFor="delegate-async-timeout-minutes" className="text-sm font-medium">
            {t("asyncDelegation.timeoutMinutes")}
          </Label>
          <Input
            id="delegate-async-timeout-minutes"
            type="number"
            inputMode="numeric"
            min={MIN_TIMEOUT_MINUTES}
            max={MAX_TIMEOUT_MINUTES}
            step={1}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            disabled={!canEdit || !connected || saving}
            aria-invalid={invalidMinutes}
            aria-describedby={invalidMinutes ? "delegate-async-timeout-hint delegate-async-timeout-validation" : "delegate-async-timeout-hint"}
            className="text-base md:text-sm"
          />
          {invalidMinutes && (
            <p id="delegate-async-timeout-validation" role="alert" className="text-sm text-destructive">
              {t("asyncDelegation.invalidMinutes")}
            </p>
          )}
        </div>
        {!canEdit && <p className="text-sm text-muted-foreground">{t("asyncDelegation.ownerOnly")}</p>}
        <p id="delegate-async-timeout-hint" className="text-sm text-muted-foreground">
          {t("asyncDelegation.asyncOnly")}
        </p>
        <p className="text-xs text-muted-foreground">{t("asyncDelegation.applyHint")}</p>
      </CardContent>
    </Card>
  );
}
