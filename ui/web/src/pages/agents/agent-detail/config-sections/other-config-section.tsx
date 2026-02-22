import { Textarea } from "@/components/ui/textarea";
import { ConfigSection } from "./config-section";

interface OtherConfigSectionProps {
  enabled: boolean;
  value: string;
  onToggle: (v: boolean) => void;
  onChange: (v: string) => void;
}

export function OtherConfigSection({ enabled, value, onToggle, onChange }: OtherConfigSectionProps) {
  return (
    <ConfigSection
      title="Other Config"
      description="Generic JSON config (no schema)"
      enabled={enabled}
      onToggle={onToggle}
    >
      <Textarea
        className="font-mono text-sm"
        rows={6}
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </ConfigSection>
  );
}
