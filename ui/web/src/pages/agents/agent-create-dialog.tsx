import { useState, useMemo } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Combobox } from "@/components/ui/combobox";
import type { AgentData } from "@/types/agent";
import { slugify, isValidSlug } from "@/lib/slug";
import { useProviders } from "@/pages/providers/hooks/use-providers";
import { useProviderModels } from "@/pages/providers/hooks/use-provider-models";
import { AGENT_PRESETS } from "./agent-presets";

interface AgentCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onCreate: (data: Partial<AgentData>) => Promise<unknown>;
}

export function AgentCreateDialog({ open, onOpenChange, onCreate }: AgentCreateDialogProps) {
  const { providers } = useProviders();
  const [agentKey, setAgentKey] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [provider, setProvider] = useState("");
  const [model, setModel] = useState("");
  const [agentType, setAgentType] = useState<"open" | "predefined">("open");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(false);

  const enabledProviders = providers.filter((p) => p.enabled);

  // Look up provider ID from selected provider name for model fetching
  const selectedProviderId = useMemo(
    () => enabledProviders.find((p) => p.name === provider)?.id,
    [enabledProviders, provider],
  );
  const { models, loading: modelsLoading } = useProviderModels(selectedProviderId);

  const handleCreate = async () => {
    if (!agentKey.trim()) return;
    setLoading(true);
    try {
      await onCreate({
        agent_key: agentKey.trim(),
        display_name: displayName.trim() || undefined,
        provider: provider.trim(),
        model: model.trim(),
        agent_type: agentType,
        other_config: description.trim() ? { description: description.trim() } : undefined,
      });
      onOpenChange(false);
      setAgentKey("");
      setDisplayName("");
      setProvider("");
      setModel("");
      setAgentType("open");
      setDescription("");
    } catch {
      // error handled upstream
    } finally {
      setLoading(false);
    }
  };

  const handleProviderChange = (value: string) => {
    setProvider(value);
    setModel("");
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Create Agent</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 py-4">
          <div className="space-y-2">
            <Label htmlFor="agentKey">Agent Key *</Label>
            <Input
              id="agentKey"
              value={agentKey}
              onChange={(e) => setAgentKey(slugify(e.target.value))}
              placeholder="e.g. my-agent"
            />
            <p className="text-xs text-muted-foreground">Lowercase letters, numbers, and hyphens only</p>
          </div>
          <div className="space-y-2">
            <Label htmlFor="displayName">Display Name</Label>
            <Input
              id="displayName"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              placeholder="My Agent"
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Provider</Label>
              {enabledProviders.length > 0 ? (
                <Select value={provider} onValueChange={handleProviderChange}>
                  <SelectTrigger>
                    <SelectValue placeholder="Select provider" />
                  </SelectTrigger>
                  <SelectContent>
                    {enabledProviders.map((p) => (
                      <SelectItem key={p.name} value={p.name}>
                        {p.display_name || p.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              ) : (
                <Input
                  value={provider}
                  onChange={(e) => setProvider(e.target.value)}
                  placeholder="openrouter"
                />
              )}
            </div>
            <div className="space-y-2">
              <Label>Model</Label>
              <Combobox
                value={model}
                onChange={setModel}
                options={models.map((m) => ({ value: m.id, label: m.name }))}
                placeholder={modelsLoading ? "Loading models..." : "Enter or select model"}
              />
            </div>
          </div>
          <div className="space-y-2">
            <Label>Agent Type</Label>
            <Select value={agentType} onValueChange={(v) => setAgentType(v as "open" | "predefined")}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="open">Open (per-user context)</SelectItem>
                <SelectItem value="predefined">Predefined (agent-level)</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {agentType === "predefined" && (
            <div className="space-y-3">
              <Label>Describe Your Agent</Label>
              <div className="flex flex-wrap gap-2">
                {AGENT_PRESETS.map((preset) => (
                  <button
                    key={preset.label}
                    type="button"
                    onClick={() => setDescription(preset.prompt)}
                    className="rounded-full border px-3 py-1 text-xs transition-colors hover:bg-accent"
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
              <Textarea
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Describe your agent's personality, purpose, and behavior..."
                className="min-h-[120px]"
              />
              <p className="text-xs text-muted-foreground">
                AI will automatically generate your agent's context files from this description.
                Leave empty to start with templates.
              </p>
            </div>
          )}
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={loading}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={!agentKey.trim() || !isValidSlug(agentKey) || loading}>
            {loading ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
