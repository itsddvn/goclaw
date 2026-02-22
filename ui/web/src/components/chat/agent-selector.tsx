import { useState, useEffect } from "react";
import { Bot, ChevronDown } from "lucide-react";
import { useWs } from "@/hooks/use-ws";
import { Methods } from "@/api/protocol";
import type { AgentInfo } from "@/types/agent";

interface AgentSelectorProps {
  value: string;
  onChange: (agentId: string) => void;
}

export function AgentSelector({ value, onChange }: AgentSelectorProps) {
  const ws = useWs();
  const [agents, setAgents] = useState<AgentInfo[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!ws.isConnected) return;
    ws.call<{ agents: AgentInfo[] }>(Methods.AGENTS_LIST)
      .then((res) => setAgents(res.agents ?? []))
      .catch(() => {});
  }, [ws]);

  const selected = agents.find((a) => a.id === value);

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="flex w-full items-center gap-2 rounded-lg border bg-background px-3 py-2 text-sm hover:bg-accent"
      >
        <Bot className="h-4 w-4 shrink-0 text-muted-foreground" />
        <span className="flex-1 truncate text-left">
          {selected?.name ?? (value || "Select agent")}
        </span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
      </button>

      {open && (
        <>
          <div className="fixed inset-0 z-40" onClick={() => setOpen(false)} />
          <div className="absolute left-0 right-0 top-full z-50 mt-1 rounded-lg border bg-popover p-1 shadow-md">
            {agents.length === 0 && (
              <div className="px-3 py-2 text-sm text-muted-foreground">
                No agents available
              </div>
            )}
            {agents.map((agent) => (
              <button
                key={agent.id}
                type="button"
                onClick={() => {
                  onChange(agent.id);
                  setOpen(false);
                }}
                className={`flex w-full items-center gap-2 rounded-md px-3 py-2 text-sm hover:bg-accent ${
                  agent.id === value ? "bg-accent" : ""
                }`}
              >
                <Bot className="h-4 w-4 shrink-0 text-muted-foreground" />
                <span className="flex-1 truncate text-left">{agent.name || agent.id}</span>
                {agent.isRunning && (
                  <span className="h-2 w-2 rounded-full bg-green-500" />
                )}
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  );
}
