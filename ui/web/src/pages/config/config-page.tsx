import { useState, useEffect } from "react";
import { Settings, Save, RefreshCw, AlertCircle, ShieldAlert } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { DetailSkeleton } from "@/components/shared/loading-skeleton";
import { useConfig } from "./hooks/use-config";
import { useMinLoading } from "@/hooks/use-min-loading";
import { GatewaySection } from "./sections/gateway-section";
import { ProvidersSection } from "./sections/providers-section";
import { AgentsDefaultsSection } from "./sections/agents-defaults-section";
import { ToolsSection } from "./sections/tools-section";
import { ChannelsSection } from "./sections/channels-section";
import { SessionsSection } from "./sections/sessions-section";
import { TtsSection } from "./sections/tts-section";
import { CronSection } from "./sections/cron-section";
import { TelemetrySection } from "./sections/telemetry-section";
import { BindingsSection } from "./sections/bindings-section";

export function ConfigPage() {
  const { config, hash, configPath, loading, saving, error, refresh, applyRaw, patch } = useConfig();
  const spinning = useMinLoading(loading);
  const [rawText, setRawText] = useState("");
  const [dirty, setDirty] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  useEffect(() => {
    if (config) {
      const text = JSON.stringify(config, null, 2);
      setRawText(text);
      setDirty(false);
      setSaveError(null);
    }
  }, [config]);

  const handleSave = async () => {
    setSaveError(null);
    try {
      await applyRaw(rawText);
      setDirty(false);
    } catch (err) {
      setSaveError(err instanceof Error ? err.message : "Failed to save");
    }
  };

  if (loading && !config) {
    return (
      <div className="p-6">
        <PageHeader title="Config" description="Gateway configuration" />
        <div className="mt-6">
          <DetailSkeleton />
        </div>
      </div>
    );
  }

  if (!config) {
    return (
      <div className="p-6">
        <PageHeader title="Config" description="Gateway configuration" />
        <div className="mt-6">
          <EmptyState
            icon={Settings}
            title="No configuration"
            description="Could not load gateway configuration."
            action={
              <Button variant="outline" size="sm" onClick={refresh}>
                Retry
              </Button>
            }
          />
        </div>
      </div>
    );
  }

  return (
    <div className="p-6">
      <PageHeader
        title="Config"
        description="Gateway configuration"
        actions={
          <div className="flex items-center gap-2">
            {configPath && (
              <span className="text-xs text-muted-foreground">{configPath}</span>
            )}
            {hash && (
              <Badge variant="outline" className="font-mono text-xs">
                {hash.slice(0, 8)}
              </Badge>
            )}
            <Button variant="outline" size="sm" onClick={refresh} disabled={spinning} className="gap-1">
              <RefreshCw className={"h-3.5 w-3.5" + (spinning ? " animate-spin" : "")} /> Refresh
            </Button>
          </div>
        }
      />

      <div className="mt-4 flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/5 px-3 py-2.5 text-sm text-amber-700 dark:text-amber-400">
        <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
        <span>
          API keys and tokens are managed via environment variables and are not shown here.
          Fields displaying <code className="rounded bg-muted px-1 font-mono text-xs">***</code> are
          read-only secrets — edit them in your <code className="rounded bg-muted px-1 font-mono text-xs">.env.local</code> file
          or server environment.
        </span>
      </div>

      <Tabs defaultValue="ui" className="mt-4">
        <TabsList>
          <TabsTrigger value="ui">UI</TabsTrigger>
          <TabsTrigger value="raw">Raw Editor</TabsTrigger>
        </TabsList>

        <TabsContent value="ui" className="mt-4">
          <ConfigUI config={config} onPatch={patch} saving={saving} />
        </TabsContent>

        <TabsContent value="raw" className="mt-4">
          <div className="space-y-3">
            <Textarea
              value={rawText}
              onChange={(e) => {
                setRawText(e.target.value);
                setDirty(true);
              }}
              className="min-h-[500px] font-mono text-sm"
              placeholder="JSON configuration..."
            />

            {(saveError || error) && (
              <div className="flex items-center gap-2 rounded-md border border-destructive/50 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                <AlertCircle className="h-4 w-4" />
                {saveError || error}
              </div>
            )}

            <div className="flex items-center gap-2">
              <Button
                onClick={handleSave}
                disabled={!dirty || saving}
                className="gap-1"
              >
                <Save className="h-3.5 w-3.5" />
                {saving ? "Saving..." : "Save"}
              </Button>
              {dirty && (
                <span className="text-xs text-muted-foreground">Unsaved changes</span>
              )}
            </div>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  );
}

/* eslint-disable @typescript-eslint/no-explicit-any */
function ConfigUI({
  config,
  onPatch,
  saving,
}: {
  config: Record<string, unknown>;
  onPatch: (updates: Record<string, unknown>) => Promise<void>;
  saving: boolean;
}) {
  return (
    <div className="space-y-4">
      <GatewaySection
        data={config.gateway as any}
        onSave={(v) => onPatch({ gateway: v })}
        saving={saving}
      />
      <ProvidersSection
        data={config.providers as any}
        onSave={(v) => onPatch({ providers: v })}
        saving={saving}
      />
      <AgentsDefaultsSection
        data={config.agents as any}
        onSave={(v) => onPatch({ agents: v })}
        saving={saving}
      />
      <ToolsSection
        data={config.tools as any}
        onSave={(v) => onPatch({ tools: v })}
        saving={saving}
      />
      <ChannelsSection
        data={config.channels as any}
        onSave={(v) => onPatch({ channels: v })}
        saving={saving}
      />
      <SessionsSection
        data={config.sessions as any}
        onSave={(v) => onPatch({ sessions: v })}
        saving={saving}
      />
      <TtsSection data={config.tts as any} />
      <CronSection
        data={config.cron as any}
        onSave={(v) => onPatch({ cron: v })}
        saving={saving}
      />
      <TelemetrySection
        data={config.telemetry as any}
        onSave={(v) => onPatch({ telemetry: v })}
        saving={saving}
      />
      <BindingsSection
        data={config.bindings as any}
        onSave={(v) => onPatch({ bindings: v })}
        saving={saving}
      />
    </div>
  );
}
