import { Bot, Star } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import type { AgentData } from "@/types/agent";

interface AgentCardProps {
  agent: AgentData;
  onClick: () => void;
}

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export function AgentCard({ agent, onClick }: AgentCardProps) {
  const displayName = agent.display_name
    || (UUID_RE.test(agent.agent_key) ? "Unnamed Agent" : agent.agent_key);

  // Show agent_key as subtitle only if there's a display_name and agent_key is meaningful
  const showSubtitle = agent.display_name && !UUID_RE.test(agent.agent_key);

  return (
    <button
      type="button"
      onClick={onClick}
      className="flex flex-col gap-3 rounded-lg border bg-card p-4 text-left transition-all hover:border-primary/30 hover:shadow-md"
    >
      {/* Top row: icon + name + status */}
      <div className="flex items-center gap-3">
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Bot className="h-4.5 w-4.5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-semibold">{displayName}</span>
            {agent.is_default && (
              <Star className="h-3.5 w-3.5 shrink-0 fill-amber-400 text-amber-400" />
            )}
          </div>
          {showSubtitle && (
            <div className="truncate text-xs text-muted-foreground">{agent.agent_key}</div>
          )}
        </div>
        <Badge variant={agent.status === "active" ? "success" : "secondary"} className="shrink-0">
          {agent.status}
        </Badge>
      </div>

      {/* Model info */}
      {(agent.provider || agent.model) && (
        <div className="truncate text-xs text-muted-foreground">
          {[agent.provider, agent.model].filter(Boolean).join(" / ")}
        </div>
      )}

      {/* Bottom badges */}
      <div className="flex items-center gap-1.5">
        <Badge variant="outline" className="text-[11px]">{agent.agent_type}</Badge>
        {agent.context_window > 0 && (
          <span className="text-[11px] text-muted-foreground">
            {(agent.context_window / 1000).toFixed(0)}K ctx
          </span>
        )}
      </div>
    </button>
  );
}
