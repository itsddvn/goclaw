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
import type { SandboxConfig } from "@/types/agent";
import { ConfigSection, numOrUndef } from "./config-section";

interface SandboxSectionProps {
  enabled: boolean;
  value: SandboxConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: SandboxConfig) => void;
}

export function SandboxSection({ enabled, value, onToggle, onChange }: SandboxSectionProps) {
  return (
    <ConfigSection
      title="Sandbox"
      description="Docker sandbox for code execution isolation"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Mode</Label>
          <Select
            value={value.mode ?? ""}
            onValueChange={(v) => onChange({ ...value, mode: v as SandboxConfig["mode"] })}
          >
            <SelectTrigger><SelectValue placeholder="off" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="off">off</SelectItem>
              <SelectItem value="non-main">non-main</SelectItem>
              <SelectItem value="all">all</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Workspace Access</Label>
          <Select
            value={value.workspace_access ?? ""}
            onValueChange={(v) =>
              onChange({ ...value, workspace_access: v as SandboxConfig["workspace_access"] })
            }
          >
            <SelectTrigger><SelectValue placeholder="rw" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="none">none</SelectItem>
              <SelectItem value="ro">ro (read-only)</SelectItem>
              <SelectItem value="rw">rw (read-write)</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      <div className="space-y-2">
        <Label>Image</Label>
        <Input
          placeholder="goclaw-sandbox:bookworm-slim"
          value={value.image ?? ""}
          onChange={(e) => onChange({ ...value, image: e.target.value || undefined })}
        />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Scope</Label>
          <Select
            value={value.scope ?? ""}
            onValueChange={(v) => onChange({ ...value, scope: v as SandboxConfig["scope"] })}
          >
            <SelectTrigger><SelectValue placeholder="session" /></SelectTrigger>
            <SelectContent>
              <SelectItem value="session">session</SelectItem>
              <SelectItem value="agent">agent</SelectItem>
              <SelectItem value="shared">shared</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="space-y-2">
          <Label>Timeout (sec)</Label>
          <Input
            type="number"
            placeholder="300"
            value={value.timeout_sec ?? ""}
            onChange={(e) => onChange({ ...value, timeout_sec: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="space-y-2">
          <Label>Memory (MB)</Label>
          <Input
            type="number"
            placeholder="512"
            value={value.memory_mb ?? ""}
            onChange={(e) => onChange({ ...value, memory_mb: numOrUndef(e.target.value) })}
          />
        </div>
        <div className="space-y-2">
          <Label>CPUs</Label>
          <Input
            type="number"
            step="0.5"
            placeholder="1.0"
            value={value.cpus ?? ""}
            onChange={(e) => onChange({ ...value, cpus: numOrUndef(e.target.value) })}
          />
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Switch
          checked={value.network_enabled ?? false}
          onCheckedChange={(v) => onChange({ ...value, network_enabled: v })}
        />
        <Label>Network Enabled</Label>
      </div>
    </ConfigSection>
  );
}
