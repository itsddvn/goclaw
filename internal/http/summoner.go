package http

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// summoningFiles is the ordered list of context files the LLM should generate.
var summoningFiles = []string{
	"SOUL.md",
	"IDENTITY.md",
	"AGENTS.md",
	"TOOLS.md",
	"HEARTBEAT.md",
}

// fileTagRe parses <file name="SOUL.md">content</file> from LLM output.
var fileTagRe = regexp.MustCompile(`(?s)<file\s+name="([^"]+)">\s*(.*?)\s*</file>`)

// AgentSummoner generates context files for predefined agents using an LLM.
// Runs one-shot background calls — no session data, no agent loop.
type AgentSummoner struct {
	agents      store.AgentStore
	providerReg *providers.Registry
	msgBus      *bus.MessageBus
}

// NewAgentSummoner creates a summoner backed by the given stores and provider registry.
func NewAgentSummoner(agents store.AgentStore, providerReg *providers.Registry, msgBus *bus.MessageBus) *AgentSummoner {
	return &AgentSummoner{
		agents:      agents,
		providerReg: providerReg,
		msgBus:      msgBus,
	}
}

// SummonAgent generates context files from a natural language description.
// Meant to be called as a goroutine: go summoner.SummonAgent(...)
// On success: stores generated files and sets agent status to "active".
// On failure: keeps template files (already seeded) and sets status to "active".
func (s *AgentSummoner) SummonAgent(agentID uuid.UUID, providerName, model, description string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	s.emitEvent(agentID, "started", "", "")

	files, err := s.generateFiles(ctx, providerName, model, s.buildCreatePrompt(description))
	if err != nil {
		slog.Warn("summoning: LLM generation failed, falling back to templates",
			"agent", agentID, "error", err)
		s.emitEvent(agentID, "failed", "", err.Error())
		s.setAgentStatus(ctx, agentID, "active")
		return
	}

	s.storeFiles(ctx, agentID, files)
	s.setAgentStatus(ctx, agentID, "active")
	s.emitEvent(agentID, "completed", "", "")

	slog.Info("summoning: completed", "agent", agentID, "files", len(files))
}

// RegenerateAgent updates context files based on an edit prompt.
// Reads existing files, sends them + edit instructions to LLM, stores results.
// Synchronous — caller should run in goroutine if needed.
func (s *AgentSummoner) RegenerateAgent(agentID uuid.UUID, providerName, model, editPrompt string) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	s.emitEvent(agentID, "started", "", "")

	// Read existing files for context
	existing, err := s.agents.GetAgentContextFiles(ctx, agentID)
	if err != nil {
		slog.Warn("summoning: failed to read existing files", "agent", agentID, "error", err)
		s.emitEvent(agentID, "failed", "", err.Error())
		s.setAgentStatus(ctx, agentID, "active")
		return
	}

	prompt := s.buildEditPrompt(existing, editPrompt)

	files, err := s.generateFiles(ctx, providerName, model, prompt)
	if err != nil {
		slog.Warn("summoning: regeneration failed", "agent", agentID, "error", err)
		s.emitEvent(agentID, "failed", "", err.Error())
		s.setAgentStatus(ctx, agentID, "active")
		return
	}

	s.storeFiles(ctx, agentID, files)
	s.setAgentStatus(ctx, agentID, "active")
	s.emitEvent(agentID, "completed", "", "")

	slog.Info("summoning: regeneration completed", "agent", agentID, "files", len(files))
}

// generateFiles calls the LLM and parses the XML-tagged response into file map.
func (s *AgentSummoner) generateFiles(ctx context.Context, providerName, model, prompt string) (map[string]string, error) {
	provider, err := s.resolveProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("resolve provider: %w", err)
	}

	resp, err := provider.Chat(ctx, providers.ChatRequest{
		Messages: []providers.Message{
			{Role: "user", Content: prompt},
		},
		Model: model,
		Options: map[string]interface{}{
			"max_tokens":  8192,
			"temperature": 0.7,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("LLM call: %w", err)
	}

	files := parseFileResponse(resp.Content)
	if len(files) == 0 {
		return nil, fmt.Errorf("LLM returned no parseable files (response length: %d)", len(resp.Content))
	}

	return files, nil
}

// storeFiles saves generated files to agent_context_files and emits progress events.
func (s *AgentSummoner) storeFiles(ctx context.Context, agentID uuid.UUID, files map[string]string) {
	for _, name := range summoningFiles {
		content, ok := files[name]
		if !ok || content == "" {
			continue
		}
		if err := s.agents.SetAgentContextFile(ctx, agentID, name, content); err != nil {
			slog.Warn("summoning: failed to store file", "agent", agentID, "file", name, "error", err)
			continue
		}
		s.emitEvent(agentID, "file_generated", name, "")
	}
}

func (s *AgentSummoner) resolveProvider(name string) (providers.Provider, error) {
	if s.providerReg == nil {
		return nil, fmt.Errorf("no provider registry")
	}

	provider, err := s.providerReg.Get(name)
	if err != nil {
		// Fallback to first available provider
		names := s.providerReg.List()
		if len(names) == 0 {
			return nil, fmt.Errorf("no providers configured")
		}
		provider, err = s.providerReg.Get(names[0])
		if err != nil {
			return nil, err
		}
		slog.Warn("summoning: provider not found, using fallback", "wanted", name, "using", names[0])
	}
	return provider, nil
}

func (s *AgentSummoner) setAgentStatus(ctx context.Context, agentID uuid.UUID, status string) {
	if err := s.agents.Update(ctx, agentID, map[string]any{"status": status}); err != nil {
		slog.Warn("summoning: failed to update agent status", "agent", agentID, "status", status, "error", err)
	}
}

func (s *AgentSummoner) emitEvent(agentID uuid.UUID, eventType, fileName, errMsg string) {
	if s.msgBus == nil {
		return
	}
	payload := map[string]interface{}{
		"type":     eventType,
		"agent_id": agentID.String(),
	}
	if fileName != "" {
		payload["file"] = fileName
	}
	if errMsg != "" {
		payload["error"] = errMsg
	}
	s.msgBus.Broadcast(bus.Event{
		Name:    protocol.EventAgentSummoning,
		Payload: payload,
	})
}

// buildCreatePrompt constructs the system + user prompt for initial file generation.
func (s *AgentSummoner) buildCreatePrompt(description string) string {
	return fmt.Sprintf(`You are setting up a new AI assistant. Based on the description below, generate the content for these context files. Write in the same language as the description.

<description>
%s
</description>

Generate each file inside XML tags. Follow the format and spirit of each file type:

- **SOUL.md**: Who the agent IS. Core personality traits, communication style, boundaries, and values. Written in first person, like a character sheet. Not a job description — a soul.
- **IDENTITY.md**: Quick identity card. Name, Creature (what kind of entity?), Vibe (how they come across), Emoji (signature), Avatar (leave blank).
- **AGENTS.md**: Operating instructions. How the agent should behave each session, handle memory, manage safety, work in groups. Keep the core structure from the template but personalize for this agent's purpose.
- **TOOLS.md**: Notes about tools relevant to this agent's role. What environment-specific details matter.
- **HEARTBEAT.md**: Periodic tasks (if the agent should check things proactively). Leave minimal/empty if not applicable.

<file name="SOUL.md">
(generate here)
</file>
<file name="IDENTITY.md">
(generate here)
</file>
<file name="AGENTS.md">
(generate here)
</file>
<file name="TOOLS.md">
(generate here)
</file>
<file name="HEARTBEAT.md">
(generate here)
</file>`, description)
}

// buildEditPrompt constructs the prompt for editing existing files.
func (s *AgentSummoner) buildEditPrompt(existing []store.AgentContextFileData, editPrompt string) string {
	var sb strings.Builder
	sb.WriteString("You are updating an existing AI assistant's configuration files.\n\nHere are the current files:\n\n<current_files>\n")
	for _, f := range existing {
		if f.Content == "" {
			continue
		}
		fmt.Fprintf(&sb, "<file name=%q>\n%s\n</file>\n", f.FileName, f.Content)
	}
	sb.WriteString("</current_files>\n\n")
	fmt.Fprintf(&sb, "<edit_instructions>\n%s\n</edit_instructions>\n\n", editPrompt)
	sb.WriteString("Generate updated files. Only include files that need changes. Keep the same XML format:\n\n")
	sb.WriteString("<file name=\"SOUL.md\">\n(updated content, or omit if unchanged)\n</file>\n")
	sb.WriteString("...\n")
	return sb.String()
}

// parseFileResponse extracts file contents from XML-tagged LLM output.
func parseFileResponse(content string) map[string]string {
	files := make(map[string]string)
	matches := fileTagRe.FindAllStringSubmatch(content, -1)
	for _, m := range matches {
		name := strings.TrimSpace(m[1])
		body := strings.TrimSpace(m[2])
		if name != "" && body != "" {
			files[name] = body
		}
	}
	return files
}
