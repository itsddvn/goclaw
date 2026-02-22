import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { SubagentsConfig } from "@/types/agent";
import { ConfigSection, numOrUndef } from "./config-section";

interface SubagentsSectionProps {
  enabled: boolean;
  value: SubagentsConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: SubagentsConfig) => void;
}

export function SubagentsSection({ enabled, value, onToggle, onChange }: SubagentsSectionProps) {
  return (
    <ConfigSection
      title="Subagents"
      description="Controls sub-agent spawning limits and behavior"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Max Concurrent</Label>
          <Input
            type="number"
            placeholder="8"
            value={value.maxConcurrent ?? ""}
            onChange={(e) => onChange({ ...value, maxConcurrent: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Max Spawn Depth</Label>
          <Select
            value={String(value.maxSpawnDepth ?? "")}
            onValueChange={(v) => onChange({ ...value, maxSpawnDepth: Number(v) })}
          >
            <SelectTrigger><SelectValue placeholder="1" /></SelectTrigger>
            <SelectContent>
              {[1, 2, 3, 4, 5].map((n) => (
                <SelectItem key={n} value={String(n)}>{n}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Max Children Per Agent</Label>
          <Input
            type="number"
            placeholder="5"
            value={value.maxChildrenPerAgent ?? ""}
            onChange={(e) => onChange({ ...value, maxChildrenPerAgent: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Archive After (minutes)</Label>
          <Input
            type="number"
            placeholder="60"
            value={value.archiveAfterMinutes ?? ""}
            onChange={(e) => onChange({ ...value, archiveAfterMinutes: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Model Override</Label>
        <Input
          placeholder="(inherit from agent)"
          value={value.model ?? ""}
          onChange={(e) => onChange({ ...value, model: e.target.value || undefined })}
        />
        <p className="text-xs text-muted-foreground">
          LLM model for sub-agents. Leave empty to inherit the parent agent's model.
        </p>
      </div>
    </ConfigSection>
  );
}
