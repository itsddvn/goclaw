import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ContextPruningConfig } from "@/types/agent";
import { ConfigSection, numOrUndef } from "./config-section";

interface ContextPruningSectionProps {
  enabled: boolean;
  value: ContextPruningConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: ContextPruningConfig) => void;
}

export function ContextPruningSection({ enabled, value, onToggle, onChange }: ContextPruningSectionProps) {
  return (
    <ConfigSection
      title="Context Pruning"
      description="Trim old tool results to save context window"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Mode</Label>
          <Select
            value={value.mode ?? ""}
            onValueChange={(v) =>
              onChange({ ...value, mode: v as ContextPruningConfig["mode"] })
            }
          >
            <SelectTrigger><SelectValue placeholder="off" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="off">off</SelectItem>
              <SelectItem value="cache-ttl">cache-ttl</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Keep Last Assistants</Label>
          <Input
            type="number"
            placeholder="3"
            value={value.keepLastAssistants ?? ""}
            onChange={(e) =>
              onChange({ ...value, keepLastAssistants: numOrUndef(e.target.value) })
            }
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Soft Trim Ratio (0-1)</Label>
          <Input
            type="number"
            step="0.05"
            placeholder="0.3"
            value={value.softTrimRatio ?? ""}
            onChange={(e) => onChange({ ...value, softTrimRatio: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Hard Clear Ratio (0-1)</Label>
          <Input
            type="number"
            step="0.05"
            placeholder="0.5"
            value={value.hardClearRatio ?? ""}
            onChange={(e) => onChange({ ...value, hardClearRatio: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label>Min Prunable Tool Chars</Label>
        <Input
          type="number"
          placeholder="50000"
          value={value.minPrunableToolChars ?? ""}
          onChange={(e) =>
            onChange({ ...value, minPrunableToolChars: numOrUndef(e.target.value) })
          }
        />
      </div>

      {/* Soft Trim */}
      <div className="space-y-3 rounded-md border border-dashed p-3">
        <h4 className="text-xs font-medium text-muted-foreground">Soft Trim</h4>
        <div className="grid grid-cols-3 gap-4">
          <div className="space-y-2">
            <Label>Max Chars</Label>
            <Input
              type="number"
              placeholder="4000"
              value={value.softTrim?.maxChars ?? ""}
              onChange={(e) =>
                onChange({ ...value, softTrim: { ...value.softTrim, maxChars: numOrUndef(e.target.value) } })
              }
            />
          </div>
          <div className="space-y-2">
            <Label>Head Chars</Label>
            <Input
              type="number"
              placeholder="1500"
              value={value.softTrim?.headChars ?? ""}
              onChange={(e) =>
                onChange({ ...value, softTrim: { ...value.softTrim, headChars: numOrUndef(e.target.value) } })
              }
            />
          </div>
          <div className="space-y-2">
            <Label>Tail Chars</Label>
            <Input
              type="number"
              placeholder="1500"
              value={value.softTrim?.tailChars ?? ""}
              onChange={(e) =>
                onChange({ ...value, softTrim: { ...value.softTrim, tailChars: numOrUndef(e.target.value) } })
              }
            />
          </div>
        </div>
        <p className="text-xs text-muted-foreground">
          Tool results longer than Max Chars get trimmed, keeping Head + Tail chars.
        </p>
      </div>

      {/* Hard Clear */}
      <div className="space-y-3 rounded-md border border-dashed p-3">
        <h4 className="text-xs font-medium text-muted-foreground">Hard Clear</h4>
        <div className="flex items-center gap-2">
          <Switch
            checked={value.hardClear?.enabled ?? true}
            onCheckedChange={(v) =>
              onChange({ ...value, hardClear: { ...value.hardClear, enabled: v } })
            }
          />
          <Label>Enabled</Label>
        </div>
        <div className="space-y-2">
          <Label>Placeholder Text</Label>
          <Input
            placeholder="[Old tool result content cleared]"
            value={value.hardClear?.placeholder ?? ""}
            onChange={(e) =>
              onChange({ ...value, hardClear: { ...value.hardClear, placeholder: e.target.value || undefined } })
            }
          />
          <p className="text-xs text-muted-foreground">
            Replacement text when old tool results are cleared entirely.
          </p>
        </div>
      </div>
    </ConfigSection>
  );
}
