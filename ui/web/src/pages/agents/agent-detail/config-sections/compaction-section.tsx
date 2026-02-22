import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { CompactionConfig } from "@/types/agent";
import { ConfigSection, numOrUndef } from "./config-section";

interface CompactionSectionProps {
  enabled: boolean;
  value: CompactionConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: CompactionConfig) => void;
}

export function CompactionSection({ enabled, value, onToggle, onChange }: CompactionSectionProps) {
  return (
    <ConfigSection
      title="Compaction"
      description="Context window compaction and memory flush settings"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Reserve Tokens Floor</Label>
          <Input
            type="number"
            placeholder="20000"
            value={value.reserveTokensFloor ?? ""}
            onChange={(e) => onChange({ ...value, reserveTokensFloor: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Max History Share (0-1)</Label>
          <Input
            type="number"
            step="0.05"
            placeholder="0.75"
            value={value.maxHistoryShare ?? ""}
            onChange={(e) => onChange({ ...value, maxHistoryShare: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="space-y-3">
        <div className="flex items-center gap-2">
          <Switch
            checked={value.memoryFlush?.enabled ?? false}
            onCheckedChange={(v) =>
              onChange({ ...value, memoryFlush: { ...value.memoryFlush, enabled: v } })
            }
          />
          <Label>Memory Flush</Label>
        </div>
        {value.memoryFlush?.enabled && (
          <div className="space-y-2 pl-6">
            <Label>Soft Threshold (tokens)</Label>
            <Input
              type="number"
              placeholder="4000"
              value={value.memoryFlush?.softThresholdTokens ?? ""}
              onChange={(e) =>
                onChange({
                  ...value,
                  memoryFlush: {
                    ...value.memoryFlush,
                    enabled: true,
                    softThresholdTokens: numOrUndef(e.target.value),
                  },
                })
              }
            />
          </div>
        )}
      </div>
    </ConfigSection>
  );
}
