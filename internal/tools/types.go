package tools

import (
	"context"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// Tool is the interface all tools must implement.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]interface{}
	Execute(ctx context.Context, args map[string]interface{}) *Result
}

// ContextualTool receives channel/chat context before execution.
type ContextualTool interface {
	Tool
	SetContext(channel, chatID string)
}

// PeerKindAware tools receive the peer kind (direct/group) before execution.
type PeerKindAware interface {
	SetPeerKind(peerKind string)
}

// SandboxAware tools receive sandbox scope key before execution.
// Used by exec tool to route commands through Docker containers.
type SandboxAware interface {
	SetSandboxKey(key string)
}

// AsyncCallback is invoked when an async tool completes.
type AsyncCallback func(ctx context.Context, result *Result)

// AsyncTool supports asynchronous execution with completion callbacks.
type AsyncTool interface {
	Tool
	SetCallback(cb AsyncCallback)
}

// --- Configuration interfaces for reducing type assertions in cmd/ wiring ---

// InterceptorAware tools can receive ContextFile and Memory interceptors (managed mode).
type InterceptorAware interface {
	SetContextFileInterceptor(*ContextFileInterceptor)
	SetMemoryInterceptor(*MemoryInterceptor)
}

// MemoryStoreAware tools can receive a MemoryStore for managed-mode queries.
type MemoryStoreAware interface {
	SetMemoryStore(store.MemoryStore)
}

// ApprovalAware tools can receive an ExecApprovalManager.
type ApprovalAware interface {
	SetApprovalManager(*ExecApprovalManager, string)
}

// PathAllowable tools can allow extra path prefixes for read access.
type PathAllowable interface {
	AllowPaths(...string)
}

// ToProviderDef converts a Tool to a providers.ToolDefinition for LLM APIs.
func ToProviderDef(t Tool) providers.ToolDefinition {
	return providers.ToolDefinition{
		Type: "function",
		Function: providers.ToolFunctionSchema{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}
