package workstation

import (
	"encoding/json"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

func TestBuildAdminUpdatesMapsCamelCaseAndPreservesSSHSecrets(t *testing.T) {
	current := &store.Workstation{
		BackendType: store.BackendSSH,
		Metadata:    []byte(`{"host":"old","port":22,"user":"deploy","privateKey":"secret"}`),
	}
	name, cwd, active := " New name ", "/srv/app", false
	metadata := json.RawMessage(`{"host":"new","port":2222,"privateKey":"","password":""}`)
	updates, err := BuildAdminUpdates(current, AdminUpdate{
		Name: &name, Active: &active, DefaultCWD: &cwd, Metadata: &metadata,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updates["name"] != "New name" || updates["default_cwd"] != cwd || updates["active"] != false {
		t.Fatalf("unexpected updates: %#v", updates)
	}
	var merged store.SSHMetadata
	if err := json.Unmarshal(updates["metadata"].([]byte), &merged); err != nil {
		t.Fatal(err)
	}
	if merged.Host != "new" || merged.Port != 2222 || merged.PrivateKey != "secret" {
		t.Fatalf("unexpected merged metadata: %#v", merged)
	}
}

func TestBuildAdminUpdatesRejectsUnknownMetadataAndEmptyRequest(t *testing.T) {
	current := &store.Workstation{
		BackendType: store.BackendSSH,
		Metadata:    []byte(`{"host":"old","port":22,"user":"deploy","password":"secret"}`),
	}
	unknown := json.RawMessage(`{"command":"rm"}`)
	if _, err := BuildAdminUpdates(current, AdminUpdate{Metadata: &unknown}); err == nil {
		t.Fatal("expected unknown metadata field rejection")
	}
	if _, err := BuildAdminUpdates(current, AdminUpdate{}); err == nil {
		t.Fatal("expected empty update rejection")
	}
}
