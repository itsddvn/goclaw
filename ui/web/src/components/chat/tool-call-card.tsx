import { Wrench, Check, AlertTriangle, Loader2 } from "lucide-react";
import type { ToolStreamEntry } from "@/types/chat";

interface ToolCallCardProps {
  entry: ToolStreamEntry;
}

export function ToolCallCard({ entry }: ToolCallCardProps) {
  return (
    <div className="my-1 flex items-center gap-2 rounded-md border bg-muted/50 px-3 py-2 text-sm">
      <ToolIcon phase={entry.phase} />
      <span className="font-medium">{entry.name}</span>
      <PhaseLabel phase={entry.phase} />
    </div>
  );
}

function ToolIcon({ phase }: { phase: ToolStreamEntry["phase"] }) {
  switch (phase) {
    case "calling":
      return <Loader2 className="h-4 w-4 animate-spin text-blue-500" />;
    case "completed":
      return <Check className="h-4 w-4 text-green-500" />;
    case "error":
      return <AlertTriangle className="h-4 w-4 text-red-500" />;
    default:
      return <Wrench className="h-4 w-4 text-muted-foreground" />;
  }
}

function PhaseLabel({ phase }: { phase: ToolStreamEntry["phase"] }) {
  const labels: Record<string, string> = {
    calling: "Running...",
    completed: "Done",
    error: "Failed",
  };
  const colors: Record<string, string> = {
    calling: "text-blue-500",
    completed: "text-green-500",
    error: "text-red-500",
  };
  return (
    <span className={`ml-auto text-xs ${colors[phase] ?? "text-muted-foreground"}`}>
      {labels[phase] ?? phase}
    </span>
  );
}
