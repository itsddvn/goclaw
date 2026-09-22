package workstation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// AdminUpdate is the public, secret-safe workstation update contract.
// Backend type and workstation key are immutable after creation.
type AdminUpdate struct {
	Name       *string          `json:"name"`
	Active     *bool            `json:"active"`
	DefaultCWD *string          `json:"defaultCwd"`
	Metadata   *json.RawMessage `json:"metadata"`
}

// BuildAdminUpdates converts the camelCase API contract into store column names.
// Metadata is shallow-merged so omitted SSH credentials remain unchanged.
func BuildAdminUpdates(current *store.Workstation, input AdminUpdate) (map[string]any, error) {
	updates := make(map[string]any)
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		updates["name"] = name
	}
	if input.Active != nil {
		updates["active"] = *input.Active
	}
	if input.DefaultCWD != nil {
		updates["default_cwd"] = *input.DefaultCWD
	}
	if input.Metadata != nil {
		merged, err := mergeMetadata(current, *input.Metadata)
		if err != nil {
			return nil, err
		}
		updates["metadata"] = merged
	}
	if len(updates) == 0 {
		return nil, fmt.Errorf("no updates provided")
	}
	return updates, nil
}

func mergeMetadata(current *store.Workstation, patch json.RawMessage) ([]byte, error) {
	var existing map[string]any
	if err := json.Unmarshal(current.Metadata, &existing); err != nil {
		return nil, fmt.Errorf("parse stored metadata: %w", err)
	}
	var incoming map[string]any
	if err := json.Unmarshal(patch, &incoming); err != nil {
		return nil, fmt.Errorf("parse metadata update: %w", err)
	}
	allowed := allowedMetadataFields(current.BackendType)
	for key, value := range incoming {
		if !allowed[key] {
			return nil, fmt.Errorf("metadata field %q is not valid for %s", key, current.BackendType)
		}
		// Empty credential inputs mean “keep the stored credential”. This lets the
		// UI edit connection details without ever reading secrets back from the API.
		if (key == "privateKey" || key == "password") && stringValueEmpty(value) {
			continue
		}
		existing[key] = value
	}
	merged, err := json.Marshal(existing)
	if err != nil {
		return nil, fmt.Errorf("encode metadata update: %w", err)
	}
	if err := store.ValidateMetadata(current.BackendType, merged); err != nil {
		return nil, err
	}
	return merged, nil
}

func allowedMetadataFields(backend store.WorkstationBackend) map[string]bool {
	switch backend {
	case store.BackendSSH:
		return map[string]bool{
			"host": true, "port": true, "user": true, "privateKey": true,
			"password": true, "knownHostsFingerprint": true, "connectTimeoutSec": true,
		}
	case store.BackendDocker:
		return map[string]bool{"host": true, "image": true, "network": true, "socketPath": true}
	default:
		return map[string]bool{}
	}
}

func stringValueEmpty(value any) bool {
	s, ok := value.(string)
	return ok && strings.TrimSpace(s) == ""
}
