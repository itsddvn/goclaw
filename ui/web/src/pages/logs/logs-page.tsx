import { useEffect, useRef } from "react";
import { Terminal, Play, Square, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { PageHeader } from "@/components/shared/page-header";
import { useLogs, type LogEntry } from "./hooks/use-logs";

const levelColors: Record<string, string> = {
  error: "text-red-500",
  warn: "text-yellow-500",
  info: "text-blue-500",
  debug: "text-muted-foreground",
};

export function LogsPage() {
  const { logs, tailing, error, startTail, stopTail, clearLogs } = useLogs();
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [logs]);

  return (
    <div className="flex h-full flex-col p-6">
      <PageHeader
        title="Logs"
        description="Live log tailing"
        actions={
          <div className="flex items-center gap-2">
            {tailing && <Badge variant="success">Live</Badge>}
            {tailing ? (
              <Button variant="outline" size="sm" onClick={stopTail} className="gap-1">
                <Square className="h-3.5 w-3.5" /> Stop
              </Button>
            ) : (
              <Button size="sm" onClick={startTail} className="gap-1">
                <Play className="h-3.5 w-3.5" /> Start Tail
              </Button>
            )}
            <Button
              variant="outline"
              size="sm"
              onClick={clearLogs}
              disabled={logs.length === 0}
              className="gap-1"
            >
              <Trash2 className="h-3.5 w-3.5" /> Clear
            </Button>
          </div>
        }
      />

      <div
        ref={scrollRef}
        className="mt-4 flex-1 overflow-y-auto rounded-md border bg-zinc-950 p-4 font-mono text-xs text-zinc-300"
      >
        {error ? (
          <div className="flex items-center justify-center py-12 text-zinc-500">
            <div className="text-center">
              <Terminal className="mx-auto mb-2 h-8 w-8" />
              <p className="text-yellow-500">{error}</p>
              <p className="mt-1 text-zinc-600">
                This feature is not yet implemented on the Go backend.
              </p>
            </div>
          </div>
        ) : logs.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-zinc-500">
            <div className="text-center">
              <Terminal className="mx-auto mb-2 h-8 w-8" />
              <p>
                {tailing
                  ? "Waiting for logs..."
                  : "Click \"Start Tail\" to begin streaming logs."}
              </p>
            </div>
          </div>
        ) : (
          logs.map((entry: LogEntry, i: number) => (
            <div key={i} className="leading-relaxed">
              <span className="text-zinc-500">
                {new Date(entry.timestamp).toLocaleTimeString()}
              </span>{" "}
              <span className={levelColors[entry.level] || "text-zinc-400"}>
                [{entry.level?.toUpperCase() || "LOG"}]
              </span>{" "}
              {entry.source && (
                <span className="text-zinc-500">[{entry.source}] </span>
              )}
              <span>{entry.message}</span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
