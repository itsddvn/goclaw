import { useState, useEffect } from "react";
import { Save } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";

interface TelemetryData {
  enabled?: boolean;
  endpoint?: string;
  protocol?: string;
  insecure?: boolean;
  service_name?: string;
  headers?: Record<string, string>;
}

const DEFAULT: TelemetryData = {};

interface Props {
  data: TelemetryData | undefined;
  onSave: (value: TelemetryData) => Promise<void>;
  saving: boolean;
}

export function TelemetrySection({ data, onSave, saving }: Props) {
  const [draft, setDraft] = useState<TelemetryData>(data ?? DEFAULT);
  const [headersText, setHeadersText] = useState("");
  const [dirty, setDirty] = useState(false);

  useEffect(() => {
    setDraft(data ?? DEFAULT);
    setHeadersText(data?.headers ? JSON.stringify(data.headers, null, 2) : "{}");
    setDirty(false);
  }, [data]);

  const update = (patch: Partial<TelemetryData>) => {
    setDraft((prev) => ({ ...prev, ...patch }));
    setDirty(true);
  };

  const handleSave = () => {
    let headers: Record<string, string> | undefined;
    try {
      headers = JSON.parse(headersText);
    } catch {
      headers = draft.headers;
    }
    onSave({ ...draft, headers });
  };

  if (!data) return null;

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-base">Telemetry</CardTitle>
        <CardDescription>OpenTelemetry export configuration</CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="flex items-center justify-between">
          <Label>Enabled</Label>
          <Switch checked={draft.enabled ?? false} onCheckedChange={(v) => update({ enabled: v })} />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="grid gap-1.5">
            <Label>Endpoint</Label>
            <Input
              value={draft.endpoint ?? ""}
              onChange={(e) => update({ endpoint: e.target.value })}
              placeholder="localhost:4317"
            />
          </div>
          <div className="grid gap-1.5">
            <Label>Protocol</Label>
            <Select value={draft.protocol ?? "grpc"} onValueChange={(v) => update({ protocol: v })}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="grpc">gRPC</SelectItem>
                <SelectItem value="http">HTTP</SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="grid gap-1.5">
            <Label>Service Name</Label>
            <Input
              value={draft.service_name ?? ""}
              onChange={(e) => update({ service_name: e.target.value })}
              placeholder="goclaw-gateway"
            />
          </div>
          <div className="flex items-center justify-between">
            <Label>Insecure (no TLS)</Label>
            <Switch checked={draft.insecure ?? false} onCheckedChange={(v) => update({ insecure: v })} />
          </div>
        </div>

        <div className="grid gap-1.5">
          <Label>Headers (JSON)</Label>
          <Textarea
            value={headersText}
            onChange={(e) => {
              setHeadersText(e.target.value);
              setDirty(true);
            }}
            className="min-h-[80px] font-mono text-xs"
            placeholder='{"Authorization": "Bearer ..."}'
          />
        </div>

        {dirty && (
          <div className="flex justify-end pt-2">
            <Button size="sm" onClick={handleSave} disabled={saving} className="gap-1.5">
              <Save className="h-3.5 w-3.5" /> {saving ? "Saving..." : "Save"}
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
