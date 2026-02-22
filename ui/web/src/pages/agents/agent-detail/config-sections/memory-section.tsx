import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import type { MemoryConfig } from "@/types/agent";
import { ConfigSection, numOrUndef } from "./config-section";

interface MemorySectionProps {
  enabled: boolean;
  value: MemoryConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: MemoryConfig) => void;
}

export function MemorySection({ enabled, value, onToggle, onChange }: MemorySectionProps) {
  return (
    <ConfigSection
      title="Memory"
      description="Semantic memory search and embedding configuration"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="flex items-center gap-2">
        <Switch
          checked={value.enabled ?? true}
          onCheckedChange={(v) => onChange({ ...value, enabled: v })}
        />
        <Label>Enabled</Label>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Embedding Provider</Label>
          <Input
            placeholder="(auto)"
            value={value.embedding_provider ?? ""}
            onChange={(e) => onChange({ ...value, embedding_provider: e.target.value || undefined })}
          />
        </div>
        <div className="space-y-2">
          <Label>Embedding Model</Label>
          <Input
            placeholder="text-embedding-3-small"
            value={value.embedding_model ?? ""}
            onChange={(e) => onChange({ ...value, embedding_model: e.target.value || undefined })}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Max Results</Label>
          <Input
            type="number"
            placeholder="6"
            value={value.max_results ?? ""}
            onChange={(e) => onChange({ ...value, max_results: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Max Chunk Length</Label>
          <Input
            type="number"
            placeholder="1000"
            value={value.max_chunk_len ?? ""}
            onChange={(e) => onChange({ ...value, max_chunk_len: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Vector Weight</Label>
          <Input
            type="number"
            step="0.1"
            placeholder="0.7"
            value={value.vector_weight ?? ""}
            onChange={(e) => onChange({ ...value, vector_weight: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>Text Weight</Label>
          <Input
            type="number"
            step="0.1"
            placeholder="0.3"
            value={value.text_weight ?? ""}
            onChange={(e) => onChange({ ...value, text_weight: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
    </ConfigSection>
  );
}
