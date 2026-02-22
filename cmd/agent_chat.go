package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/sessions"
	"github.com/nextlevelbuilder/goclaw/internal/skills"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/file"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func agentChatCmd() *cobra.Command {
	var (
		agentName  string
		message    string
		sessionKey string
	)

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Chat with an agent interactively or send a one-shot message",
		Long: `Chat with an agent via the running gateway (WebSocket client mode).
Falls back to standalone mode if the gateway is not running.

Examples:
  goclaw agent chat                          # Interactive REPL
  goclaw agent chat --name coder             # Chat with "coder" agent
  goclaw agent chat -m "What time is it?"    # One-shot message
  goclaw agent chat -s my-session            # Continue a session`,
		Run: func(cmd *cobra.Command, args []string) {
			runAgentChat(agentName, message, sessionKey)
		},
	}

	cmd.Flags().StringVarP(&agentName, "name", "n", "default", "agent name")
	cmd.Flags().StringVarP(&message, "message", "m", "", "one-shot message (omit for interactive mode)")
	cmd.Flags().StringVarP(&sessionKey, "session", "s", "", "session key (default: auto-generated)")

	return cmd
}

func runAgentChat(agentName, message, sessionKey string) {
	cfgPath := resolveConfigPath()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Default session key
	if sessionKey == "" {
		sessionKey = sessions.BuildSessionKey(agentName, "cli", sessions.PeerDirect, "local")
	}

	// Try client mode first (connect to running gateway)
	host := cfg.Gateway.Host
	if host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	addr := fmt.Sprintf("%s:%d", host, cfg.Gateway.Port)

	if isGatewayRunning(addr) {
		fmt.Fprintf(os.Stderr, "Connected to gateway at %s\n", addr)
		runClientMode(cfg, addr, agentName, message, sessionKey)
		return
	}

	// Fallback: standalone mode
	fmt.Fprintf(os.Stderr, "Gateway not running, using standalone mode\n")
	runStandaloneMode(cfg, agentName, message, sessionKey)
}

// --- Gateway detection ---

func isGatewayRunning(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// ============================================================
// CLIENT MODE — connect to running gateway via WebSocket
// ============================================================

func runClientMode(cfg *config.Config, addr, agentName, message, sessionKey string) {
	wsURL := fmt.Sprintf("ws://%s/ws", addr)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WebSocket connect failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "Falling back to standalone mode\n")
		runStandaloneMode(cfg, agentName, message, sessionKey)
		return
	}
	defer conn.Close()

	// Authenticate
	if err := wsConnect(conn, cfg.Gateway.Token); err != nil {
		fmt.Fprintf(os.Stderr, "Gateway auth failed: %v\n", err)
		os.Exit(1)
	}

	agentCfg := cfg.ResolveAgent(agentName)

	if message != "" {
		// One-shot mode
		resp, err := wsChatSend(conn, agentName, sessionKey, message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)
		return
	}

	// Interactive REPL
	fmt.Fprintf(os.Stderr, "\nGoClaw Interactive Chat (agent: %s, model: %s)\n", agentName, agentCfg.Model)
	fmt.Fprintf(os.Stderr, "Session: %s\n", sessionKey)
	fmt.Fprintf(os.Stderr, "Type \"exit\" to quit, \"/new\" for new session\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Fprintln(os.Stderr, "Goodbye!")
			return
		}
		if input == "/new" {
			sessionKey = sessions.BuildSessionKey(agentName, "cli", sessions.PeerDirect, uuid.NewString()[:8])
			fmt.Fprintf(os.Stderr, "New session: %s\n\n", sessionKey)
			continue
		}

		resp, err := wsChatSend(conn, agentName, sessionKey, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			continue
		}
		fmt.Printf("\n%s\n\n", resp)
	}
}

// wsConnect sends the connect RPC and waits for auth response.
func wsConnect(conn *websocket.Conn, token string) error {
	params := map[string]string{}
	if token != "" {
		params["token"] = token
	}
	paramsJSON, _ := json.Marshal(params)

	reqFrame := protocol.RequestFrame{
		Type:   protocol.FrameTypeRequest,
		ID:     "connect-1",
		Method: protocol.MethodConnect,
		Params: paramsJSON,
	}

	if err := conn.WriteJSON(reqFrame); err != nil {
		return fmt.Errorf("send connect: %w", err)
	}

	var resp protocol.ResponseFrame
	if err := conn.ReadJSON(&resp); err != nil {
		return fmt.Errorf("read connect response: %w", err)
	}
	if !resp.OK {
		if resp.Error != nil {
			return fmt.Errorf("connect rejected: %s", resp.Error.Message)
		}
		return fmt.Errorf("connect rejected")
	}

	return nil
}

// wsChatSend sends a chat.send RPC and waits for the response,
// displaying events (tool calls, chunks) in real-time.
func wsChatSend(conn *websocket.Conn, agentID, sessionKey, message string) (string, error) {
	reqID := uuid.NewString()[:8]
	params, _ := json.Marshal(map[string]interface{}{
		"message":    message,
		"agentId":    agentID,
		"sessionKey": sessionKey,
		"stream":     true,
	})

	reqFrame := protocol.RequestFrame{
		Type:   protocol.FrameTypeRequest,
		ID:     reqID,
		Method: protocol.MethodChatSend,
		Params: params,
	}

	if err := conn.WriteJSON(reqFrame); err != nil {
		return "", fmt.Errorf("send chat: %w", err)
	}

	// Read frames until we get our response
	var finalContent string
	for {
		_, rawMsg, err := conn.ReadMessage()
		if err != nil {
			return "", fmt.Errorf("read: %w", err)
		}

		frameType, _ := protocol.ParseFrameType(rawMsg)

		switch frameType {
		case protocol.FrameTypeResponse:
			var resp protocol.ResponseFrame
			if err := json.Unmarshal(rawMsg, &resp); err != nil {
				continue
			}
			if resp.ID != reqID {
				continue // response for a different request
			}
			if !resp.OK {
				if resp.Error != nil {
					return "", fmt.Errorf("agent error: %s", resp.Error.Message)
				}
				return "", fmt.Errorf("agent error (unknown)")
			}
			// Extract content from payload
			if payload, ok := resp.Payload.(map[string]interface{}); ok {
				if content, ok := payload["content"].(string); ok && content != "" {
					finalContent = content
				}
			}
			return finalContent, nil

		case protocol.FrameTypeEvent:
			var evt protocol.EventFrame
			if err := json.Unmarshal(rawMsg, &evt); err != nil {
				continue
			}
			handleCLIEvent(evt)
		}
	}
}

// handleCLIEvent displays agent events in the terminal.
func handleCLIEvent(evt protocol.EventFrame) {
	payload, ok := evt.Payload.(map[string]interface{})
	if !ok {
		return
	}

	evtType, _ := payload["type"].(string)

	switch evt.Event {
	case protocol.EventAgent:
		switch evtType {
		case protocol.AgentEventToolCall:
			if p, ok := payload["payload"].(map[string]interface{}); ok {
				name, _ := p["toolName"].(string)
				if name == "" {
					name, _ = p["name"].(string)
				}
				fmt.Fprintf(os.Stderr, "  [tool] %s\n", name)
			}
		case protocol.AgentEventToolResult:
			if p, ok := payload["payload"].(map[string]interface{}); ok {
				isErr, _ := p["is_error"].(bool)
				name, _ := p["toolName"].(string)
				if name == "" {
					name, _ = p["name"].(string)
				}
				if isErr {
					fmt.Fprintf(os.Stderr, "  [tool] %s -> error\n", name)
				}
			}
		}

	case protocol.EventChat:
		switch evtType {
		case protocol.ChatEventChunk:
			if content, ok := payload["content"].(string); ok {
				fmt.Print(content)
			}
		}
	}
}

// ============================================================
// STANDALONE MODE — bootstrap mini agent loop
// ============================================================

func runStandaloneMode(cfg *config.Config, agentName, message, sessionKey string) {
	loop, sessStore, agentCfg := bootstrapStandaloneAgent(cfg, agentName)

	chatFn := func(msg string) (string, error) {
		runID := fmt.Sprintf("cli-%s", uuid.NewString()[:8])
		result, err := loop.Run(context.Background(), agent.RunRequest{
			SessionKey: sessionKey,
			Message:    msg,
			Channel:    "cli",
			ChatID:     "local",
			PeerKind:   "direct",
			RunID:      runID,
		})
		if err != nil {
			return "", err
		}
		return result.Content, nil
	}

	_ = sessStore // keep reference for session persistence

	if message != "" {
		resp, err := chatFn(message)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(resp)
		return
	}

	// Interactive REPL
	fmt.Fprintf(os.Stderr, "\nGoClaw Interactive Chat — Standalone Mode\n")
	fmt.Fprintf(os.Stderr, "Agent: %s | Model: %s\n", agentName, agentCfg.Model)
	fmt.Fprintf(os.Stderr, "Session: %s\n", sessionKey)
	fmt.Fprintf(os.Stderr, "Type \"exit\" to quit, \"/new\" for new session\n\n")

	// Handle Ctrl+C gracefully
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	scanner := bufio.NewScanner(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(os.Stderr, "\nGoodbye!")
			return
		default:
		}

		fmt.Fprint(os.Stderr, "You: ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "exit" || input == "quit" {
			fmt.Fprintln(os.Stderr, "Goodbye!")
			return
		}
		if input == "/new" {
			sessionKey = sessions.BuildSessionKey(agentName, "cli", sessions.PeerDirect, uuid.NewString()[:8])
			fmt.Fprintf(os.Stderr, "New session: %s\n\n", sessionKey)
			continue
		}

		resp, err := chatFn(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n\n", err)
			continue
		}
		fmt.Printf("\n%s\n\n", resp)
	}
}

// bootstrapStandaloneAgent creates a minimal agent loop for CLI usage.
func bootstrapStandaloneAgent(cfg *config.Config, agentName string) (*agent.Loop, store.SessionStore, config.AgentDefaults) {
	agentCfg := cfg.ResolveAgent(agentName)
	workspace := config.ExpandHome(agentCfg.Workspace)
	if !filepath.IsAbs(workspace) {
		workspace, _ = filepath.Abs(workspace)
	}

	// Ensure workspace exists
	os.MkdirAll(workspace, 0755)

	// 1. Provider
	providerReg := providers.NewRegistry()
	registerProviders(providerReg, cfg)

	provider, err := providerReg.Get(agentCfg.Provider)
	if err != nil {
		names := providerReg.List()
		if len(names) == 0 {
			fmt.Fprintf(os.Stderr, "Error: no providers configured. Run 'goclaw onboard' first.\n")
			os.Exit(1)
		}
		provider, _ = providerReg.Get(names[0])
		slog.Warn("configured provider not found, using fallback", "wanted", agentCfg.Provider, "using", names[0])
	}

	// 2. Sessions (wrap file-based manager in store adapter)
	sessStorage := config.ExpandHome(cfg.Sessions.Storage)
	sessStore := file.NewFileSessionStore(sessions.NewManager(sessStorage))

	// 3. Tools
	toolsReg := tools.NewRegistry()
	toolsReg.Register(tools.NewReadFileTool(workspace, agentCfg.RestrictToWorkspace))
	toolsReg.Register(tools.NewWriteFileTool(workspace, agentCfg.RestrictToWorkspace))
	toolsReg.Register(tools.NewListFilesTool(workspace, agentCfg.RestrictToWorkspace))
	toolsReg.Register(tools.NewExecTool(workspace, agentCfg.RestrictToWorkspace))

	// Web tools
	webSearchTool := tools.NewWebSearchTool(tools.WebSearchConfig{
		BraveEnabled: cfg.Tools.Web.Brave.Enabled,
		BraveAPIKey:  cfg.Tools.Web.Brave.APIKey,
		DDGEnabled:   cfg.Tools.Web.DuckDuckGo.Enabled,
	})
	if webSearchTool != nil {
		toolsReg.Register(webSearchTool)
	}
	toolsReg.Register(tools.NewWebFetchTool(tools.WebFetchConfig{}))

	// 4. Bootstrap files
	rawFiles := bootstrap.LoadWorkspaceFiles(workspace)
	truncCfg := bootstrap.TruncateConfig{
		MaxCharsPerFile: agentCfg.BootstrapMaxChars,
		TotalMaxChars:   agentCfg.BootstrapTotalMaxChars,
	}
	if truncCfg.MaxCharsPerFile <= 0 {
		truncCfg.MaxCharsPerFile = bootstrap.DefaultMaxCharsPerFile
	}
	if truncCfg.TotalMaxChars <= 0 {
		truncCfg.TotalMaxChars = bootstrap.DefaultTotalMaxChars
	}
	contextFiles := bootstrap.BuildContextFiles(rawFiles, truncCfg)

	// 5. Skills
	globalSkillsDir := filepath.Join(config.ExpandHome("~/.goclaw"), "skills")
	skillsLoader := skills.NewLoader(workspace, globalSkillsDir, "")
	toolsReg.Register(tools.NewSkillSearchTool(skillsLoader))

	// Allow read_file to access skills directories
	if readTool, ok := toolsReg.Get("read_file"); ok {
		if rt, ok := readTool.(*tools.ReadFileTool); ok {
			rt.AllowPaths(globalSkillsDir)
			if homeDir, err := os.UserHomeDir(); err == nil {
				rt.AllowPaths(filepath.Join(homeDir, ".agents", "skills"))
			}
		}
	}

	// 6. Event display (tool calls on stderr)
	var eventMu sync.Mutex
	onEvent := func(evt agent.AgentEvent) {
		eventMu.Lock()
		defer eventMu.Unlock()

		switch evt.Type {
		case protocol.AgentEventToolCall:
			if p, ok := evt.Payload.(map[string]interface{}); ok {
				name, _ := p["name"].(string)
				fmt.Fprintf(os.Stderr, "  [tool] %s\n", name)
			}
		case protocol.AgentEventToolResult:
			// silent — avoid noisy output
		}
	}

	// Per-agent skill allowlist
	var skillAllowList []string
	if spec, ok := cfg.Agents.List[agentName]; ok {
		skillAllowList = spec.Skills
	}

	// 7. Create agent loop
	msgBus := bus.New()
	loop := agent.NewLoop(agent.LoopConfig{
		ID:            agentName,
		Provider:      provider,
		Model:         agentCfg.Model,
		ContextWindow: agentCfg.ContextWindow,
		MaxIterations: agentCfg.MaxToolIterations,
		Workspace:     workspace,
		Bus:           msgBus,
		Sessions:      sessStore,
		Tools:         toolsReg,
		OnEvent:       onEvent,
		OwnerIDs:      cfg.Gateway.OwnerIDs,
		SkillsLoader:  skillsLoader,
		SkillAllowList: skillAllowList,
		HasMemory:     false, // skip memory for standalone CLI (avoids SQLite dep issues)
		ContextFiles:  contextFiles,
		CompactionCfg:     agentCfg.Compaction,
		ContextPruningCfg: agentCfg.ContextPruning,
	})

	return loop, sessStore, agentCfg
}
