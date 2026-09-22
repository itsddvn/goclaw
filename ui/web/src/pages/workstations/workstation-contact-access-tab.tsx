import { useCallback, useEffect, useMemo, useState } from "react";
import { ContactRound, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useTranslation } from "react-i18next";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { useHttp } from "@/hooks/use-ws";
import type { ChannelContact } from "@/types/contact";
import { grantContactsInBatch } from "./workstation-contact-access-helpers";
import { WorkstationContactMultiSelect } from "./workstation-contact-multi-select";
import type { WorkstationContactGrant } from "./hooks/use-workstations";

interface WorkstationContactAccessTabProps {
  workstationId: string;
  onList: (workstationId: string) => Promise<WorkstationContactGrant[]>;
  onGrant: (workstationId: string, contactId: string) => Promise<void>;
  onRevoke: (workstationId: string, contactId: string) => Promise<void>;
}

export function WorkstationContactAccessTab({
  workstationId,
  onList,
  onGrant,
  onRevoke,
}: WorkstationContactAccessTabProps) {
  const { t } = useTranslation("workstations");
  const http = useHttp();
  const [grants, setGrants] = useState<WorkstationContactGrant[]>([]);
  const [contacts, setContacts] = useState<ChannelContact[]>([]);
  const [selectedContacts, setSelectedContacts] = useState<ChannelContact[]>([]);
  const [contactSearch, setContactSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [contactsLoading, setContactsLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      setGrants(await onList(workstationId));
    } catch (err) {
      setGrants([]);
      setError(err instanceof Error ? err.message : t("contactAccess.failedLoad"));
    } finally {
      setLoading(false);
    }
  }, [onList, workstationId, t]);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    let cancelled = false;
    setContactsLoading(true);
    const timer = window.setTimeout(async () => {
      try {
        const params: Record<string, string> = { contact_type: "user", limit: "50" };
        if (contactSearch.trim()) params.search = contactSearch.trim();
        const result = await http.get<{ contacts: ChannelContact[] }>("/v1/contacts", params);
        if (!cancelled) setContacts(result.contacts ?? []);
      } catch {
        if (!cancelled) setContacts([]);
      } finally {
        if (!cancelled) setContactsLoading(false);
      }
    }, 150);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [contactSearch, http]);

  const grantedIds = useMemo(() => new Set(grants.map((grant) => grant.contactId)), [grants]);
  const availableContacts = useMemo(
    () => contacts.filter((contact) => !grantedIds.has(contact.id)),
    [contacts, grantedIds],
  );

  useEffect(() => {
    setSelectedContacts((current) => {
      const available = current.filter((contact) => !grantedIds.has(contact.id));
      return available.length === current.length ? current : available;
    });
  }, [grantedIds]);

  function handleToggleContact(contact: ChannelContact) {
    setSelectedContacts((current) => current.some((item) => item.id === contact.id)
      ? current.filter((item) => item.id !== contact.id)
      : [...current, contact]);
  }

  async function runMutation(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : t("contactAccess.failedSave"));
    } finally {
      setBusy(false);
    }
  }

  async function handleGrant() {
    if (selectedContacts.length === 0) {
      setError(t("contactAccess.contactRequired"));
      return;
    }
    setBusy(true);
    setError("");
    const selectedIds = selectedContacts.map((contact) => contact.id);
    try {
      const result = await grantContactsInBatch(
        selectedIds,
        (contactId) => onGrant(workstationId, contactId),
      );
      const succeeded = new Set(result.succeededIds);
      setSelectedContacts((current) => current.filter((contact) => !succeeded.has(contact.id)));
      await load();
      if (result.failedIds.length > 0) {
        setError(t("contactAccess.partialFailure", {
          succeeded: result.succeededIds.length,
          total: selectedIds.length,
          failed: result.failedIds.length,
        }));
      } else {
        setContactSearch("");
      }
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="space-y-4 p-1 sm:p-4">
      <Alert>
        <ContactRound />
        <AlertTitle>{t("contactAccess.policyTitle")}</AlertTitle>
        <AlertDescription>{t("contactAccess.policyDescription")}</AlertDescription>
      </Alert>

      <div className="flex flex-col gap-2 sm:flex-row">
        <WorkstationContactMultiSelect
          contacts={availableContacts}
          selectedContacts={selectedContacts}
          search={contactSearch}
          onSearchChange={setContactSearch}
          onToggle={handleToggleContact}
          placeholder={t("contactAccess.selectContact")}
          loadingText={t("contactAccess.searching")}
          noResultsText={t("contactAccess.noAvailableContacts")}
          removeSelectionLabel={(contact) => t("contactAccess.removeSelection", { contact })}
          disabled={busy}
          loading={contactsLoading}
        />
        <Button
          size="sm"
          onClick={() => void handleGrant()}
          disabled={busy || selectedContacts.length === 0}
          className="gap-1 sm:self-start"
        >
          <Plus className="h-3.5 w-3.5" />
          {t("contactAccess.grantSelected", { count: selectedContacts.length })}
        </Button>
      </div>

      {loading ? (
        <p className="text-sm text-muted-foreground">{t("contactAccess.loading")}</p>
      ) : grants.length === 0 ? (
        <div className="rounded-md border border-dashed p-6 text-center">
          <p className="text-sm font-medium">{t("contactAccess.emptyTitle")}</p>
          <p className="mt-1 text-xs text-muted-foreground">{t("contactAccess.emptyDescription")}</p>
        </div>
      ) : (
        <div className="divide-y rounded-md border">
          {grants.map((grant) => {
            const displayName = grant.displayName || grant.username || grant.senderId;
            return (
              <div key={grant.contactId} className="flex items-center gap-3 px-3 py-2.5">
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{displayName}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {grant.channelType} · {grant.senderId}
                  </p>
                </div>
                <Button
                  variant="ghost"
                  size="icon-sm"
                  onClick={() => void runMutation(() => onRevoke(workstationId, grant.contactId))}
                  disabled={busy}
                  aria-label={t("contactAccess.revoke", { contact: displayName })}
                >
                  <Trash2 className="h-3.5 w-3.5 text-destructive" />
                </Button>
              </div>
            );
          })}
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
