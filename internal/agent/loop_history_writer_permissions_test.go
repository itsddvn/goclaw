package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/bootstrap"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type writerPromptPermissionStore struct {
	writers       []store.ConfigPermission
	allowed       bool
	checkedScope  string
	checkedType   string
	checkedUserID string
}

func (s *writerPromptPermissionStore) CheckPermission(_ context.Context, _ uuid.UUID, scope, configType, userID string) (bool, error) {
	s.checkedScope = scope
	s.checkedType = configType
	s.checkedUserID = userID
	return s.allowed, nil
}

func (*writerPromptPermissionStore) Grant(context.Context, *store.ConfigPermission) error {
	return nil
}

func (*writerPromptPermissionStore) Revoke(context.Context, uuid.UUID, string, string, string) error {
	return nil
}

func (s *writerPromptPermissionStore) List(context.Context, uuid.UUID, string, string) ([]store.ConfigPermission, error) {
	return s.writers, nil
}

func (s *writerPromptPermissionStore) ListFileWriters(context.Context, uuid.UUID, string) ([]store.ConfigPermission, error) {
	return s.writers, nil
}

func TestBuildGroupWriterPromptUsesEffectivePermission(t *testing.T) {
	agentID := uuid.New()
	groupID := "group:tele-quangia:-5322360047"
	permStore := &writerPromptPermissionStore{
		allowed: true,
		writers: []store.ConfigPermission{{
			UserID:   "6446561210",
			Metadata: json.RawMessage(`{"displayName":"Ái Lâm"}`),
		}},
	}
	loop := &Loop{agentUUID: agentID, configPermStore: permStore}
	ctx := writerPromptContext(agentID, groupID, "1191288445|itsddvn", "Duc Nguyen CPPAI")
	files := []bootstrap.ContextFile{
		{Path: bootstrap.SoulFile},
		{Path: bootstrap.AgentsFile},
		{Path: bootstrap.IdentityFile},
	}

	prompt, gotFiles := loop.buildGroupWriterPrompt(ctx, groupID, "1191288445|itsddvn", files)

	if !strings.Contains(prompt, "CURRENT SENDER IS A FILE WRITER (Duc Nguyen CPPAI, ID: 1191288445)") {
		t.Fatalf("effective wildcard permission must authorize sender, prompt:\n%s", prompt)
	}
	if strings.Contains(prompt, "IS NOT A FILE WRITER") {
		t.Fatalf("authorized sender received refusal prompt:\n%s", prompt)
	}
	if len(gotFiles) != len(files) {
		t.Fatalf("authorized sender lost context files: got %d, want %d", len(gotFiles), len(files))
	}
	if permStore.checkedScope != groupID || permStore.checkedType != store.ConfigTypeFileWriter || permStore.checkedUserID != "1191288445" {
		t.Fatalf("effective permission check = (%q, %q, %q), want (%q, %q, %q)",
			permStore.checkedScope, permStore.checkedType, permStore.checkedUserID,
			groupID, store.ConfigTypeFileWriter, "1191288445")
	}
}

func TestBuildGroupWriterPromptHonorsEffectiveDeny(t *testing.T) {
	agentID := uuid.New()
	groupID := "group:telegram:-100123"
	permStore := &writerPromptPermissionStore{
		allowed: false,
		writers: []store.ConfigPermission{{
			UserID:   "1191288445",
			Metadata: json.RawMessage(`{"displayName":"Duc Nguyen CPPAI"}`),
		}},
	}
	loop := &Loop{agentUUID: agentID, configPermStore: permStore}
	ctx := writerPromptContext(agentID, groupID, "1191288445", "Duc Nguyen CPPAI")
	files := []bootstrap.ContextFile{
		{Path: bootstrap.SoulFile},
		{Path: bootstrap.AgentsFile},
		{Path: bootstrap.IdentityFile},
	}

	prompt, gotFiles := loop.buildGroupWriterPrompt(ctx, groupID, "1191288445", files)

	if !strings.Contains(prompt, "CURRENT SENDER (ID: 1191288445) IS NOT A FILE WRITER") {
		t.Fatalf("effective deny must reject sender, prompt:\n%s", prompt)
	}
	if len(gotFiles) != 1 || gotFiles[0].Path != bootstrap.IdentityFile {
		t.Fatalf("denied sender context files = %+v, want only %s", gotFiles, bootstrap.IdentityFile)
	}
}

func writerPromptContext(agentID uuid.UUID, groupID, senderID, senderName string) context.Context {
	ctx := context.Background()
	ctx = store.WithTenantID(ctx, store.MasterTenantID)
	ctx = store.WithAgentID(ctx, agentID)
	ctx = store.WithUserID(ctx, groupID)
	ctx = store.WithSenderID(ctx, senderID)
	ctx = store.WithSenderName(ctx, senderName)
	return ctx
}
