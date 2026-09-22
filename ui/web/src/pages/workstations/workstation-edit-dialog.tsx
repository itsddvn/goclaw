import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { UpdateWorkstationParams, Workstation } from "./hooks/use-workstations";
import {
  buildWorkstationEditPayload,
  editFormFromWorkstation,
  type WorkstationEditFormState,
} from "./workstation-management-helpers";

interface WorkstationEditDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workstation: Workstation;
  onUpdate: (params: UpdateWorkstationParams) => Promise<void>;
}

export function WorkstationEditDialog({
  open,
  onOpenChange,
  workstation,
  onUpdate,
}: WorkstationEditDialogProps) {
  const { t } = useTranslation("workstations");
  const [form, setForm] = useState<WorkstationEditFormState>(() =>
    editFormFromWorkstation(workstation),
  );
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!open) return;
    setForm(editFormFromWorkstation(workstation));
    setError("");
  }, [open, workstation]);

  const setField = <K extends keyof WorkstationEditFormState>(
    field: K,
    value: WorkstationEditFormState[K],
  ) => setForm((current) => ({ ...current, [field]: value }));

  async function handleSubmit(event: FormEvent) {
    event.preventDefault();
    const result = buildWorkstationEditPayload(form);
    if (result.kind === "error") {
      setError(t(`editDialog.errors.${result.errorKey}`));
      return;
    }
    setSubmitting(true);
    setError("");
    try {
      await onUpdate(result.payload);
      onOpenChange(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : t("editDialog.errors.updateFailed"));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!submitting) onOpenChange(next); }}>
      <DialogContent className="max-h-[90dvh] overflow-y-auto sm:max-w-xl">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{t("editDialog.title", { name: workstation.name })}</DialogTitle>
            <DialogDescription>{t("editDialog.description")}</DialogDescription>
          </DialogHeader>

          <div className="mt-4 space-y-4">
            <div className="space-y-1.5">
              <Label htmlFor="edit-ws-name">{t("editDialog.nameLabel")}</Label>
              <Input
                id="edit-ws-name"
                value={form.name}
                onChange={(event) => setField("name", event.target.value)}
                className="text-base md:text-sm"
                required
              />
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="edit-ws-cwd">{t("editDialog.defaultCwdLabel")}</Label>
              <Input
                id="edit-ws-cwd"
                value={form.defaultCwd}
                onChange={(event) => setField("defaultCwd", event.target.value)}
                placeholder={t("editDialog.defaultCwdPlaceholder")}
                className="text-base md:text-sm"
              />
            </div>

            <div className="flex items-start justify-between gap-4 rounded-md border p-3">
              <div>
                <Label htmlFor="edit-ws-active">{t("editDialog.activeLabel")}</Label>
                <p className="mt-0.5 text-xs text-muted-foreground">{t("editDialog.activeHint")}</p>
              </div>
              <Switch
                id="edit-ws-active"
                checked={form.active}
                onCheckedChange={(checked) => setField("active", checked)}
              />
            </div>

            <div className="rounded-md border p-3">
              <p className="mb-3 text-sm font-medium">
                {t("editDialog.connectionTitle", { backend: t(`backend.${form.backend}`) })}
              </p>
              {form.backend === "ssh" ? (
                <div className="space-y-3">
                  <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
                    <div className="space-y-1.5 sm:col-span-2">
                      <Label htmlFor="edit-ws-host">{t("editDialog.hostLabel")}</Label>
                      <Input id="edit-ws-host" value={form.host} onChange={(event) => setField("host", event.target.value)} className="text-base md:text-sm" />
                    </div>
                    <div className="space-y-1.5">
                      <Label htmlFor="edit-ws-port">{t("editDialog.portLabel")}</Label>
                      <Input id="edit-ws-port" type="number" min={1} max={65535} value={form.port} onChange={(event) => setField("port", event.target.value)} className="text-base md:text-sm" />
                    </div>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-user">{t("editDialog.userLabel")}</Label>
                    <Input id="edit-ws-user" value={form.user} onChange={(event) => setField("user", event.target.value)} className="text-base md:text-sm" />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-private-key">{t("editDialog.privateKeyLabel")}</Label>
                    <Textarea id="edit-ws-private-key" value={form.privateKey} onChange={(event) => setField("privateKey", event.target.value)} rows={4} spellCheck={false} autoComplete="off" className="font-mono text-base md:text-sm" />
                    <p className="text-xs text-muted-foreground">{t("editDialog.secretHint")}</p>
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-password">{t("editDialog.passwordLabel")}</Label>
                    <Input id="edit-ws-password" type="password" value={form.password} onChange={(event) => setField("password", event.target.value)} autoComplete="new-password" className="text-base md:text-sm" />
                    <p className="text-xs text-muted-foreground">{t("editDialog.secretHint")}</p>
                  </div>
                </div>
              ) : (
                <div className="space-y-3">
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-container">{t("editDialog.containerLabel")}</Label>
                    <Input id="edit-ws-container" value={form.container} onChange={(event) => setField("container", event.target.value)} className="text-base md:text-sm" />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-image">{t("editDialog.imageLabel")}</Label>
                    <Input id="edit-ws-image" value={form.image} onChange={(event) => setField("image", event.target.value)} className="text-base md:text-sm" />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="edit-ws-socket">{t("editDialog.socketPathLabel")}</Label>
                    <Input id="edit-ws-socket" value={form.socketPath} onChange={(event) => setField("socketPath", event.target.value)} className="text-base md:text-sm" />
                  </div>
                </div>
              )}
            </div>

            {error && <p className="text-sm text-destructive">{error}</p>}
          </div>

          <DialogFooter className="mt-6">
            <Button type="button" variant="outline" onClick={() => onOpenChange(false)} disabled={submitting}>
              {t("editDialog.cancel")}
            </Button>
            <Button type="submit" disabled={submitting}>{t("editDialog.save")}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
