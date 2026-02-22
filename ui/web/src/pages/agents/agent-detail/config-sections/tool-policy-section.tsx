import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ToolPolicyConfig } from "@/types/agent";
import { ConfigSection, arrayToTags, tagsToArray } from "./config-section";

interface ToolPolicySectionProps {
  enabled: boolean;
  value: ToolPolicyConfig;
  onToggle: (v: boolean) => void;
  onChange: (v: ToolPolicyConfig) => void;
}

export function ToolPolicySection({ enabled, value, onToggle, onChange }: ToolPolicySectionProps) {
  return (
    <ConfigSection
      title="Tool Policy"
      description="Control which tools this agent can use"
      enabled={enabled}
      onToggle={onToggle}
    >
      <div className="space-y-2">
        <Label>Profile</Label>
        <Select
          value={value.profile ?? ""}
          onValueChange={(v) => onChange({ ...value, profile: v || undefined })}
        >
          <SelectTrigger><SelectValue placeholder="default" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="default">default</SelectItem>
            <SelectItem value="strict">strict</SelectItem>
            <SelectItem value="permissive">permissive</SelectItem>
          </SelectContent>
        </Select>
      </div>
      <div className="space-y-2">
        <Label>Allow</Label>
        <Input
          placeholder="tool1, tool2, ..."
          value={arrayToTags(value.allow)}
          onChange={(e) => onChange({ ...value, allow: tagsToArray(e.target.value) })}
        />
        <p className="text-xs text-muted-foreground">Comma-separated list of allowed tools.</p>
      </div>
      <div className="space-y-2">
        <Label>Deny</Label>
        <Input
          placeholder="tool1, tool2, ..."
          value={arrayToTags(value.deny)}
          onChange={(e) => onChange({ ...value, deny: tagsToArray(e.target.value) })}
        />
      </div>
      <div className="space-y-2">
        <Label>Also Allow</Label>
        <Input
          placeholder="web_fetch, web_search, ..."
          value={arrayToTags(value.alsoAllow)}
          onChange={(e) => onChange({ ...value, alsoAllow: tagsToArray(e.target.value) })}
        />
        <p className="text-xs text-muted-foreground">Additional tools to add on top of profile defaults.</p>
      </div>
    </ConfigSection>
  );
}
