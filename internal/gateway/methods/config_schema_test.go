package methods

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func TestConfigSchemaIncludesSystemMessageDefinitions(t *testing.T) {
	t.Parallel()

	methods := NewConfigMethods(config.Default(), "", nil, nil)
	client, responses := gateway.NewCapturingTestClient(permissions.RoleOwner, store.MasterTenantID, "owner", 1)

	methods.handleSchema(
		store.WithTenantID(context.Background(), store.MasterTenantID),
		client,
		&protocol.RequestFrame{
			Type:   protocol.FrameTypeRequest,
			ID:     "schema-system-messages",
			Method: protocol.MethodConfigSchema,
		},
	)

	res := readConfigSchemaResponse(t, responses)
	if !res.OK {
		t.Fatalf("config.schema failed: %#v", res.Error)
	}
	raw, err := json.Marshal(res.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		JSON struct {
			Properties map[string]struct {
				Properties  map[string]any `json:"properties"`
				Definitions []struct {
					Key          string            `json:"key"`
					Template     string            `json:"template"`
					Labels       map[string]string `json:"labels"`
					Descriptions map[string]string `json:"descriptions"`
					Variables    []string          `json:"variables"`
				} `json:"definitions"`
			} `json:"properties"`
		} `json:"json"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	defs := payload.JSON.Properties["system_messages"].Definitions
	if len(defs) == 0 {
		t.Fatal("system_messages definitions missing from config.schema")
	}
	if defs[0].Key == "" || defs[0].Template == "" || len(defs[0].Variables) == 0 {
		t.Fatalf("system_messages definition incomplete: %#v", defs[0])
	}
	foundLocalizedMetadata := false
	for _, def := range defs {
		if def.Key == "pairing.account_required" {
			if def.Labels["vi"] == "" || def.Descriptions["vi"] == "" {
				t.Fatalf("pairing.account_required missing Vietnamese metadata: %#v", def)
			}
			foundLocalizedMetadata = true
		}
	}
	if !foundLocalizedMetadata {
		t.Fatal("pairing.account_required definition missing from config.schema")
	}
	if _, ok := payload.JSON.Properties["system_messages"].Properties["default_locale"]; !ok {
		t.Fatal("system_messages.default_locale missing from config.schema")
	}
}

func TestConfigSchemaIncludesDelegateAsyncTimeout(t *testing.T) {
	t.Parallel()

	methods := NewConfigMethods(config.Default(), "", nil, nil)
	client, responses := gateway.NewCapturingTestClient(permissions.RoleOwner, store.MasterTenantID, "owner", 1)
	methods.handleSchema(
		store.WithTenantID(context.Background(), store.MasterTenantID),
		client,
		&protocol.RequestFrame{
			Type:   protocol.FrameTypeRequest,
			ID:     "schema-delegate-async-timeout",
			Method: protocol.MethodConfigSchema,
		},
	)

	res := readConfigSchemaResponse(t, responses)
	if !res.OK {
		t.Fatalf("config.schema failed: %#v", res.Error)
	}
	raw, err := json.Marshal(res.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		JSON struct {
			Properties map[string]struct {
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"properties"`
		} `json:"json"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	timeoutSchema, ok := payload.JSON.Properties["tools"].Properties["delegateAsyncTimeoutSeconds"]
	if !ok {
		t.Fatal("tools.delegateAsyncTimeoutSeconds missing from config.schema")
	}
	var timeout struct {
		Type    string `json:"type"`
		AnyOf   []struct {
			Const   *int `json:"const"`
			Minimum *int `json:"minimum"`
			Maximum *int `json:"maximum"`
		} `json:"anyOf"`
		Default int `json:"default"`
	}
	if err := json.Unmarshal(timeoutSchema, &timeout); err != nil {
		t.Fatal(err)
	}
	if timeout.Type != "integer" ||
		timeout.Default != config.DefaultDelegateAsyncTimeoutSeconds ||
		len(timeout.AnyOf) != 2 ||
		timeout.AnyOf[0].Const == nil ||
		*timeout.AnyOf[0].Const != 0 ||
		timeout.AnyOf[1].Minimum == nil ||
		*timeout.AnyOf[1].Minimum != config.MinDelegateAsyncTimeoutSeconds ||
		timeout.AnyOf[1].Maximum == nil ||
		*timeout.AnyOf[1].Maximum != config.MaxDelegateAsyncTimeoutSeconds {
		t.Fatalf("delegate async timeout schema = %#v", timeout)
	}
}

func readConfigSchemaResponse(t *testing.T, responses <-chan []byte) protocol.ResponseFrame {
	t.Helper()
	select {
	case raw := <-responses:
		var res protocol.ResponseFrame
		if err := json.Unmarshal(raw, &res); err != nil {
			t.Fatal(err)
		}
		return res
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for config.schema response")
		return protocol.ResponseFrame{}
	}
}
