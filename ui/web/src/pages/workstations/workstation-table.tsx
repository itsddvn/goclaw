import { Fragment } from "react";
import {
  ChevronDown,
  ChevronRight,
  LoaderCircle,
  Pencil,
  PlugZap,
  Trash2,
  Users,
} from "lucide-react";
import { useTranslation } from "react-i18next";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { formatDate } from "@/lib/format";
import type {
  Workstation,
  WorkstationPermission,
  WorkstationContactGrant,
} from "./hooks/use-workstations";
import { WorkstationActivityTab } from "./workstation-activity-tab";
import { WorkstationCommandPermissionsTab } from "./workstation-command-permissions-tab";
import { WorkstationContactAccessTab } from "./workstation-contact-access-tab";

interface WorkstationTableProps {
  workstations: Workstation[];
  expandedId: string | null;
  testingIds: Set<string>;
  onToggleExpand: (id: string) => void;
  onEdit: (workstation: Workstation) => void;
  onTest: (workstation: Workstation) => void;
  onAssignAgents: (workstation: Workstation) => void;
  onDelete: (workstation: Workstation) => void;
  onListPermissions: (workstationId: string) => Promise<WorkstationPermission[]>;
  onAddPermission: (workstationId: string, pattern: string) => Promise<void>;
  onRemovePermission: (workstationId: string, permissionId: string) => Promise<void>;
  onTogglePermission: (workstationId: string, permissionId: string, enabled: boolean) => Promise<void>;
  onListContactGrants: (workstationId: string) => Promise<WorkstationContactGrant[]>;
  onGrantContact: (workstationId: string, contactId: string) => Promise<void>;
  onRevokeContact: (workstationId: string, contactId: string) => Promise<void>;
}

export function WorkstationTable(props: WorkstationTableProps) {
  const { t } = useTranslation("workstations");

  return (
    <div className="overflow-x-auto rounded-md border">
      <table className="w-full min-w-[800px] text-sm">
        <thead>
          <tr className="border-b bg-muted/50">
            <th className="w-8 px-4 py-3 text-left font-medium" />
            <th className="px-4 py-3 text-left font-medium">{t("columns.name")}</th>
            <th className="px-4 py-3 text-left font-medium">{t("columns.key")}</th>
            <th className="px-4 py-3 text-left font-medium">{t("columns.backend")}</th>
            <th className="px-4 py-3 text-left font-medium">{t("columns.status")}</th>
            <th className="px-4 py-3 text-left font-medium">{t("columns.created")}</th>
            <th className="px-4 py-3 text-right font-medium">{t("columns.actions")}</th>
          </tr>
        </thead>
        <tbody>
          {props.workstations.map((workstation) => {
            const expanded = props.expandedId === workstation.id;
            const testing = props.testingIds.has(workstation.id);
            return (
              <Fragment key={workstation.id}>
                <tr className="cursor-pointer border-b hover:bg-muted/30" onClick={() => props.onToggleExpand(workstation.id)}>
                  <td className="px-4 py-3 text-muted-foreground">
                    {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                  </td>
                  <td className="px-4 py-3 font-medium">{workstation.name}</td>
                  <td className="px-4 py-3 font-mono text-xs text-muted-foreground">{workstation.workstationKey}</td>
                  <td className="px-4 py-3"><Badge variant="outline">{t(`backend.${workstation.backendType}`)}</Badge></td>
                  <td className="px-4 py-3">
                    <Badge variant={workstation.active ? "default" : "secondary"}>
                      {workstation.active ? t("status.active") : t("status.inactive")}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-muted-foreground">{formatDate(new Date(workstation.createdAt))}</td>
                  <td className="px-4 py-3 text-right" onClick={(event) => event.stopPropagation()}>
                    <div className="flex justify-end gap-1">
                      <Button variant="ghost" size="sm" onClick={() => props.onEdit(workstation)} className="gap-1">
                        <Pencil className="h-3.5 w-3.5" />{t("actions.edit")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => props.onTest(workstation)} disabled={testing} className="gap-1">
                        {testing ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <PlugZap className="h-3.5 w-3.5" />}
                        {t("actions.test")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => props.onAssignAgents(workstation)} className="gap-1">
                        <Users className="h-3.5 w-3.5" />{t("actions.assignAgents")}
                      </Button>
                      <Button variant="ghost" size="sm" onClick={() => props.onDelete(workstation)} className="gap-1">
                        <Trash2 className="h-3.5 w-3.5" />{t("actions.delete")}
                      </Button>
                    </div>
                  </td>
                </tr>
                {expanded && (
                  <tr className="bg-muted/10">
                    <td colSpan={7} className="px-4 py-4">
                      <Tabs defaultValue="activity">
                        <div className="mb-3 overflow-x-auto pb-1">
                          <TabsList>
                            <TabsTrigger value="activity">{t("activity.title")}</TabsTrigger>
                            <TabsTrigger value="permissions">{t("permissions.title")}</TabsTrigger>
                            <TabsTrigger value="contacts">{t("contactAccess.title")}</TabsTrigger>
                          </TabsList>
                        </div>
                        <TabsContent value="activity"><WorkstationActivityTab workstationId={workstation.id} /></TabsContent>
                        <TabsContent value="permissions">
                          <WorkstationCommandPermissionsTab
                            workstationId={workstation.id}
                            onList={props.onListPermissions}
                            onAdd={props.onAddPermission}
                            onRemove={props.onRemovePermission}
                            onToggle={props.onTogglePermission}
                          />
                        </TabsContent>
                        <TabsContent value="contacts">
                          <WorkstationContactAccessTab
                            workstationId={workstation.id}
                            onList={props.onListContactGrants}
                            onGrant={props.onGrantContact}
                            onRevoke={props.onRevokeContact}
                          />
                        </TabsContent>
                      </Tabs>
                    </td>
                  </tr>
                )}
              </Fragment>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}
