package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestExecTrustedDelegationUsesHostDespiteSandboxAndAllowsHostWorkingDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}

	ctx, _, outputs := delegationArtifactToolContext(t)
	tenantID := uuid.New()
	agentID := uuid.New()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithAgentID(ctx, agentID)
	ctx = WithToolSandboxKey(ctx, "delegated-sandbox-must-not-run")

	hostDir := t.TempDir()
	const contents = "trusted-delegated-host-read"
	if err := os.WriteFile(filepath.Join(hostDir, "probe-input"), []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(hostDir, "probe-marker")

	manager := &recordingSandboxManager{}
	tool := NewSandboxedExecTool(outputs, true, manager)
	tool.SetTrustedDelegationHostAgents([]config.TrustedDelegationHostAgent{{
		TenantID: tenantID.String(),
		AgentID:  agentID.String(),
	}})

	result := tool.Execute(ctx, map[string]any{
		"command":     "cat probe-input && touch " + strconv.Quote(marker),
		"working_dir": hostDir,
	})

	if result.IsError {
		t.Fatalf("trusted delegated host exec failed: %s", result.ForLLM)
	}
	if !strings.Contains(result.ForLLM, contents) {
		t.Fatalf("trusted delegated host read output = %q, want %q", result.ForLLM, contents)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("trusted delegated host marker was not created: %v", err)
	}
	if manager.sandbox != nil || manager.key != "" {
		t.Fatalf("trusted delegated exec used sandbox manager: sandbox=%#v key=%q", manager.sandbox, manager.key)
	}
}

func TestExecTrustedDelegationRequiresExactIdentityAndDefaultsToDeny(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}

	tenantID := uuid.New()
	agentID := uuid.New()
	grants := []config.TrustedDelegationHostAgent{{
		TenantID: tenantID.String(),
		AgentID:  agentID.String(),
	}}

	for _, tt := range []struct {
		name      string
		tenantID  uuid.UUID
		agentID   uuid.UUID
		configure bool
	}{
		{name: "wrong agent", tenantID: tenantID, agentID: uuid.New(), configure: true},
		{name: "wrong tenant", tenantID: uuid.New(), agentID: agentID, configure: true},
		{name: "no configured grant", tenantID: tenantID, agentID: agentID},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _, outputs := delegationArtifactToolContext(t)
			ctx = store.WithTenantID(ctx, tt.tenantID)
			ctx = store.WithAgentID(ctx, tt.agentID)
			marker := filepath.Join(t.TempDir(), "must-not-run")

			tool := NewExecTool(outputs, true)
			if tt.configure {
				tool.SetTrustedDelegationHostAgents(grants)
			}
			result := tool.Execute(ctx, map[string]any{
				"command": "touch " + strconv.Quote(marker),
			})

			if !result.IsError || result.ForLLM != delegatedExecSandboxRequiredError {
				t.Fatalf("untrusted delegated exec = %#v, want stable sandbox-required error", result)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("untrusted delegated exec reached host; marker stat error = %v", err)
			}
		})
	}
}

func TestExecTrustedDelegationClearingGrantsRestoresDefaultDeny(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}

	ctx, _, outputs := delegationArtifactToolContext(t)
	tenantID := uuid.New()
	agentID := uuid.New()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithAgentID(ctx, agentID)
	marker := filepath.Join(t.TempDir(), "probe-marker")

	tool := NewExecTool(outputs, true)
	tool.SetTrustedDelegationHostAgents([]config.TrustedDelegationHostAgent{{
		TenantID: tenantID.String(),
		AgentID:  agentID.String(),
	}})
	if result := tool.Execute(ctx, map[string]any{"command": "touch " + strconv.Quote(marker)}); result.IsError {
		t.Fatalf("trusted delegated exec before clear failed: %s", result.ForLLM)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}

	tool.SetTrustedDelegationHostAgents(nil)
	result := tool.Execute(ctx, map[string]any{"command": "touch " + strconv.Quote(marker)})

	if !result.IsError || result.ForLLM != delegatedExecSandboxRequiredError {
		t.Fatalf("delegated exec after grant clear = %#v, want stable sandbox-required error", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("cleared trusted grant still reached host; marker stat error = %v", err)
	}
}

func TestExecTrustedDelegationStillAppliesSafetyDeny(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell command fixture")
	}

	ctx, _, outputs := delegationArtifactToolContext(t)
	tenantID := uuid.New()
	agentID := uuid.New()
	ctx = store.WithTenantID(ctx, tenantID)
	ctx = store.WithAgentID(ctx, agentID)
	marker := filepath.Join(t.TempDir(), "must-remain")
	if err := os.WriteFile(marker, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}

	tool := NewExecTool(outputs, true)
	tool.SetTrustedDelegationHostAgents([]config.TrustedDelegationHostAgent{{
		TenantID: tenantID.String(),
		AgentID:  agentID.String(),
	}})
	result := tool.Execute(ctx, map[string]any{"command": "rm -rf " + strconv.Quote(marker)})

	if !result.IsError || !strings.Contains(result.ForLLM, "command denied by safety policy") {
		t.Fatalf("trusted delegated unsafe command = %#v, want safety deny", result)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("safety-denied trusted command changed host marker: %v", err)
	}
}
