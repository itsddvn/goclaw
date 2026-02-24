import { useState, useEffect } from "react";
import { FileText, Save, Info, Lock, Sparkles } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog";
import { useAuthStore } from "@/stores/use-auth-store";
import type { AgentData, BootstrapFile } from "@/types/agent";

interface AgentFilesTabProps {
  agent: AgentData;
  files: BootstrapFile[];
  onGetFile: (name: string) => Promise<BootstrapFile | null>;
  onSetFile: (name: string, content: string) => Promise<void>;
  onRegenerate?: (prompt: string) => Promise<void>;
}

const FILE_DESCRIPTIONS: Record<string, string> = {
  "AGENTS.md": "Operating instructions and agent behavior rules",
  "SOUL.md": "Persona, tone, and behavioral boundaries",
  "TOOLS.md": "Notes about available tools and their usage",
  "IDENTITY.md": "Agent name, emoji, avatar, and description",
  "USER.md": "User profile (per-user customizable)",
  "HEARTBEAT.md": "Periodic tasks configuration",
  "BOOTSTRAP.md": "First-run ritual (auto-deleted after completion)",
  "MEMORY.json": "Long-term memory data",
};

export function AgentFilesTab({ agent, files, onGetFile, onSetFile, onRegenerate }: AgentFilesTabProps) {
  const userId = useAuthStore((s) => s.userId);
  const [selectedFile, setSelectedFile] = useState<string | null>(null);
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [dirty, setDirty] = useState(false);
  const [regenerateOpen, setRegenerateOpen] = useState(false);
  const [regeneratePrompt, setRegeneratePrompt] = useState("");
  const [regenerating, setRegenerating] = useState(false);

  const isOpen = agent.agent_type === "open";
  const isPredefined = agent.agent_type === "predefined";
  const isOwner = agent.owner_id === userId;
  const canEdit = !isPredefined || isOwner;

  // Hide MEMORY.json — memory is managed by the memory system, not context files
  const displayFiles = files.filter((f) => f.name !== "MEMORY.json");
  const allMissing = displayFiles.length > 0 && displayFiles.every((f) => f.missing);

  // USER.md is per-user at runtime — not editable from the agent admin UI for predefined agents
  const isUserScoped = (fileName: string) => isPredefined && fileName === "USER.md";

  useEffect(() => {
    if (!selectedFile) return;
    setLoading(true);
    onGetFile(selectedFile)
      .then((file) => {
        setContent(file?.content ?? "");
        setDirty(false);
      })
      .finally(() => setLoading(false));
  }, [selectedFile, onGetFile]);

  const handleSave = async () => {
    if (!selectedFile) return;
    setSaving(true);
    try {
      await onSetFile(selectedFile, content);
      setDirty(false);
    } finally {
      setSaving(false);
    }
  };

  // For open agents with no agent-level files, show explanation
  if (isOpen && allMissing) {
    return (
      <div className="max-w-2xl space-y-4">
        <div className="flex items-start gap-3 rounded-lg border border-info/30 bg-sky-500/5 p-4">
          <Info className="mt-0.5 h-5 w-5 shrink-0 text-sky-600 dark:text-sky-400" />
          <div className="space-y-2 text-sm">
            <p className="font-medium">Open Agent - Per-User Context Files</p>
            <p className="text-muted-foreground">
              This is an <strong>open</strong> agent. Context files (AGENTS.md, SOUL.md, TOOLS.md, etc.)
              are personalized for each user. They are automatically created from templates when a user
              first chats with this agent.
            </p>
            <p className="text-muted-foreground">
              Agent-level files shown here are empty because open agents store all context per-user
              in the <code className="rounded bg-muted px-1 py-0.5 text-xs">user_context_files</code> table.
            </p>
          </div>
        </div>

        <div className="rounded-lg border p-4">
          <h4 className="mb-3 text-sm font-medium">Context Files</h4>
          <div className="space-y-2">
            {displayFiles.map((file) => (
              <div key={file.name} className="flex items-center gap-3 rounded-md bg-muted/50 px-3 py-2">
                <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="text-sm font-medium">{file.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {FILE_DESCRIPTIONS[file.name] || "Context file"}
                  </div>
                </div>
                <Badge variant="outline" className="shrink-0 text-[10px]">per-user</Badge>
              </div>
            ))}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {isPredefined && !isOwner && (
        <div className="flex items-start gap-3 rounded-lg border border-amber-500/30 bg-amber-500/5 p-4">
          <Lock className="mt-0.5 h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400" />
          <div className="text-sm">
            <p className="font-medium">Read-only</p>
            <p className="text-muted-foreground">
              Only the agent owner can edit predefined context files.
            </p>
          </div>
        </div>
      )}

      {isPredefined && isOwner && onRegenerate && (
        <div className="flex justify-end">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setRegenerateOpen(true)}
            className="gap-1.5"
          >
            <Sparkles className="h-3.5 w-3.5" />
            Edit with AI
          </Button>
        </div>
      )}

      <div className="flex h-[600px] gap-4">
        {/* File list */}
        <div className="w-52 space-y-1 overflow-y-auto border-r pr-4">
          {displayFiles.map((file) => {
            const userScoped = isUserScoped(file.name);
            return (
              <button
                key={file.name}
                type="button"
                onClick={() => !userScoped && setSelectedFile(file.name)}
                disabled={userScoped}
                className={`flex w-full items-center gap-2 rounded-md px-2 py-2 text-sm transition-colors ${
                  userScoped
                    ? "cursor-not-allowed opacity-60"
                    : selectedFile === file.name
                      ? "bg-accent text-accent-foreground"
                      : "hover:bg-muted"
                }`}
              >
                <FileText className="h-3.5 w-3.5 shrink-0" />
                <span className="min-w-0 flex-1 truncate text-left">{file.name}</span>
                {userScoped ? (
                  <Badge variant="outline" className="shrink-0 text-[10px]">per-user</Badge>
                ) : file.missing ? (
                  <Badge variant="outline" className="shrink-0 text-[10px]">empty</Badge>
                ) : (
                  <span className="shrink-0 text-[10px] text-muted-foreground">
                    {file.size > 1024 ? `${(file.size / 1024).toFixed(1)}K` : `${file.size}B`}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {/* Editor */}
        <div className="flex flex-1 flex-col">
          {selectedFile ? (
            <>
              <div className="mb-2 flex items-center justify-between">
                <div>
                  <span className="text-sm font-medium">{selectedFile}</span>
                  {FILE_DESCRIPTIONS[selectedFile] && (
                    <span className="ml-2 text-xs text-muted-foreground">
                      - {FILE_DESCRIPTIONS[selectedFile]}
                    </span>
                  )}
                </div>
                {canEdit && (
                  <Button
                    size="sm"
                    onClick={handleSave}
                    disabled={!dirty || saving}
                  >
                    {!saving && <Save className="h-3.5 w-3.5" />}
                    {saving ? "Saving..." : "Save"}
                  </Button>
                )}
              </div>
              {loading && !content ? (
                <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
                  Loading...
                </div>
              ) : (
                <Textarea
                  value={content}
                  onChange={(e) => {
                    if (!canEdit) return;
                    setContent(e.target.value);
                    setDirty(true);
                  }}
                  readOnly={!canEdit}
                  className={`flex-1 resize-none font-mono text-sm ${!canEdit ? "opacity-70" : ""}`}
                  placeholder="File content..."
                />
              )}
            </>
          ) : (
            <div className="flex flex-1 items-center justify-center text-sm text-muted-foreground">
              Select a file to {canEdit ? "edit" : "view"}
            </div>
          )}
        </div>
      </div>

      {/* Edit with AI dialog */}
      <Dialog open={regenerateOpen} onOpenChange={setRegenerateOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Sparkles className="h-4 w-4" />
              Edit with AI
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-3 py-2">
            <p className="text-sm text-muted-foreground">
              Describe what you want to change. AI will read the current files and update them accordingly.
            </p>
            <Textarea
              value={regeneratePrompt}
              onChange={(e) => setRegeneratePrompt(e.target.value)}
              placeholder="e.g. Make the agent more formal, add Vietnamese language support, change the name to Luna..."
              className="min-h-[100px]"
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setRegenerateOpen(false)} disabled={regenerating}>
              Cancel
            </Button>
            <Button
              onClick={async () => {
                if (!onRegenerate || !regeneratePrompt.trim()) return;
                setRegenerating(true);
                try {
                  await onRegenerate(regeneratePrompt.trim());
                  setRegenerateOpen(false);
                  setRegeneratePrompt("");
                } finally {
                  setRegenerating(false);
                }
              }}
              disabled={!regeneratePrompt.trim() || regenerating}
              className="gap-1.5"
            >
              <Sparkles className="h-3.5 w-3.5" />
              {regenerating ? "Sending..." : "Regenerate"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
