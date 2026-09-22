import { useState } from "react";
import { MonitorCog, Plus, RefreshCw } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { ConfirmDialog } from "@/components/shared/confirm-dialog";
import { useMinLoading } from "@/hooks/use-min-loading";
import { useDeferredLoading } from "@/hooks/use-deferred-loading";
import { userFriendlyError } from "@/lib/error-utils";
import { toast } from "@/stores/use-toast-store";
import { useWorkstations, type Workstation } from "./hooks/use-workstations";
import { WorkstationCreateDialog } from "./workstation-create-dialog";
import { WorkstationGrantsDialog } from "./workstation-grants-dialog";
import { WorkstationEditDialog } from "./workstation-edit-dialog";
import { WorkstationTable } from "./workstation-table";

export function WorkstationsPage() {
  const { t } = useTranslation("workstations");
  const {
    workstations,
    loading,
    refresh,
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
  } = useWorkstations();

  const spinning = useMinLoading(loading);
  const isEmpty = workstations.length === 0;
  const showSkeleton = useDeferredLoading(loading && isEmpty);

  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Workstation | null>(null);
  const [editTarget, setEditTarget] = useState<Workstation | null>(null);
  const [grantsTarget, setGrantsTarget] = useState<Workstation | null>(null);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [testingIds, setTestingIds] = useState<Set<string>>(() => new Set());

  function toggleExpand(id: string) {
    setExpandedId((prev) => (prev === id ? null : id));
  }

  async function handleTest(workstation: Workstation) {
    setTestingIds((current) => new Set(current).add(workstation.id));
    try {
      const ok = await testWorkstation(workstation.id);
      if (!ok) throw new Error(t("testResult.failed"));
      toast.success(t("testResult.success"), workstation.name);
    } catch (err) {
      toast.error(t("testResult.failed"), userFriendlyError(err));
    } finally {
      setTestingIds((current) => {
        const next = new Set(current);
        next.delete(workstation.id);
        return next;
      });
    }
  }

  return (
    <div className="p-4 sm:p-6 pb-10">
      <PageHeader
        title={t("title")}
        description={t("description")}
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} />
              {t("common:refresh", "Refresh")}
            </Button>
            <Button size="sm" onClick={() => setCreateOpen(true)} className="gap-1">
              <Plus className="h-3.5 w-3.5" />
              {t("addWorkstation")}
            </Button>
          </div>
        }
      />

      <div className="mt-4">
        {showSkeleton ? (
          <TableSkeleton rows={4} />
        ) : isEmpty ? (
          <EmptyState
            icon={MonitorCog}
            title={t("emptyTitle")}
            description={t("emptyDescription")}
          />
        ) : (
          <WorkstationTable
            workstations={workstations}
            expandedId={expandedId}
            testingIds={testingIds}
            onToggleExpand={toggleExpand}
            onEdit={setEditTarget}
            onTest={(workstation) => void handleTest(workstation)}
            onAssignAgents={setGrantsTarget}
            onDelete={setDeleteTarget}
            onListPermissions={listPermissions}
            onAddPermission={addPermission}
            onRemovePermission={removePermission}
            onTogglePermission={togglePermission}
            onListContactGrants={listContactGrants}
            onGrantContact={grantContact}
            onRevokeContact={revokeContact}
          />
        )}
      </div>

      <WorkstationCreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        onCreate={async (params) => {
          await createWorkstation(params);
        }}
      />

      {editTarget && (
        <WorkstationEditDialog
          open
          onOpenChange={(open) => { if (!open) setEditTarget(null); }}
          workstation={editTarget}
          onUpdate={(params) => updateWorkstation(editTarget.id, params)}
        />
      )}

      {grantsTarget && (
        <WorkstationGrantsDialog
          open
          onOpenChange={(open) => { if (!open) setGrantsTarget(null); }}
          workstation={grantsTarget}
          onLoadGrants={listAgentGrants}
          onGrant={(agentId, isDefault) => grantAgent(grantsTarget.id, agentId, isDefault)}
          onRevoke={(agentId) => revokeAgent(grantsTarget.id, agentId)}
        />
      )}

      {deleteTarget && (
        <ConfirmDialog
          open
          onOpenChange={() => setDeleteTarget(null)}
          title={t("deleteDialog.title")}
          description={t("deleteDialog.description", { name: deleteTarget.name })}
          confirmLabel={t("deleteDialog.confirmLabel")}
          variant="destructive"
          onConfirm={async () => {
            await deleteWorkstation(deleteTarget.id);
            setDeleteTarget(null);
          }}
        />
      )}
    </div>
  );
}
