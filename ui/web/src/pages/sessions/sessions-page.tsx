import { useState } from "react";
import { useParams, useNavigate } from "react-router";
import { History } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { EmptyState } from "@/components/shared/empty-state";
import { SearchInput } from "@/components/shared/search-input";
import { Badge } from "@/components/ui/badge";
import { TableSkeleton } from "@/components/shared/loading-skeleton";
import { useSessions } from "./hooks/use-sessions";
import { SessionDetailPage } from "./session-detail-page";
import { parseSessionKey } from "@/lib/session-key";
import { formatRelativeTime } from "@/lib/format";
import type { SessionInfo } from "@/types/session";

export function SessionsPage() {
  const { key: detailKey } = useParams<{ key: string }>();
  const navigate = useNavigate();
  const { sessions, loading, preview, deleteSession, resetSession } = useSessions();
  const [search, setSearch] = useState("");

  const detailSession = detailKey
    ? sessions.find((s) => s.key === decodeURIComponent(detailKey))
    : null;

  if (detailSession) {
    return (
      <SessionDetailPage
        session={detailSession}
        onBack={() => navigate("/sessions")}
        onPreview={preview}
        onDelete={async (key) => {
          await deleteSession(key);
          navigate("/sessions");
        }}
        onReset={resetSession}
      />
    );
  }

  const filtered = sessions.filter((s) => {
    const q = search.toLowerCase();
    return s.key.toLowerCase().includes(q) || (s.label ?? "").toLowerCase().includes(q);
  });

  return (
    <div className="p-6">
      <PageHeader title="Sessions" description="Browse conversation sessions" />

      <div className="mt-4">
        <SearchInput
          value={search}
          onChange={setSearch}
          placeholder="Search sessions..."
          className="max-w-sm"
        />
      </div>

      <div className="mt-6">
        {loading && sessions.length === 0 ? (
          <TableSkeleton rows={8} />
        ) : filtered.length === 0 ? (
          <EmptyState
            icon={History}
            title={search ? "No matching sessions" : "No sessions yet"}
            description={
              search
                ? "Try a different search term."
                : "Sessions will appear here once you start chatting."
            }
          />
        ) : (
          <div className="rounded-md border">
            <table className="w-full">
              <thead>
                <tr className="border-b bg-muted/50">
                  <th className="px-4 py-3 text-left text-sm font-medium">Session</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">Agent</th>
                  <th className="px-4 py-3 text-left text-sm font-medium">Channel</th>
                  <th className="px-4 py-3 text-right text-sm font-medium">Messages</th>
                  <th className="px-4 py-3 text-right text-sm font-medium">Updated</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((session) => (
                  <SessionRow
                    key={session.key}
                    session={session}
                    onClick={() => navigate(`/sessions/${encodeURIComponent(session.key)}`)}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function SessionRow({
  session,
  onClick,
}: {
  session: SessionInfo;
  onClick: () => void;
}) {
  const parsed = parseSessionKey(session.key);

  return (
    <tr
      className="cursor-pointer border-b transition-colors hover:bg-muted/50"
      onClick={onClick}
    >
      <td className="px-4 py-3">
        <div className="text-sm font-medium">{session.label || parsed.scope}</div>
        <div className="text-xs text-muted-foreground">{session.key}</div>
      </td>
      <td className="px-4 py-3">
        <Badge variant="outline">{parsed.agentId}</Badge>
      </td>
      <td className="px-4 py-3 text-sm text-muted-foreground">
        {session.channel || "ws"}
      </td>
      <td className="px-4 py-3 text-right text-sm">{session.messageCount}</td>
      <td className="px-4 py-3 text-right text-sm text-muted-foreground">
        {formatRelativeTime(session.updated)}
      </td>
    </tr>
  );
}
