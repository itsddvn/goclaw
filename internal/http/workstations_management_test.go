package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type workstationManagementStore struct {
	store.WorkstationStore
	workstation *store.Workstation
	updates     map[string]any
}

func (s *workstationManagementStore) GetByID(_ context.Context, id uuid.UUID) (*store.Workstation, error) {
	if s.workstation == nil || s.workstation.ID != id {
		return nil, nil
	}
	copy := *s.workstation
	if s.updates != nil {
		if name, ok := s.updates["name"].(string); ok {
			copy.Name = name
		}
		if active, ok := s.updates["active"].(bool); ok {
			copy.Active = active
		}
		if cwd, ok := s.updates["default_cwd"].(string); ok {
			copy.DefaultCWD = cwd
		}
		if metadata, ok := s.updates["metadata"].([]byte); ok {
			copy.Metadata = metadata
		}
	}
	return &copy, nil
}

func (s *workstationManagementStore) Update(_ context.Context, _ uuid.UUID, updates map[string]any) error {
	s.updates = updates
	return nil
}

func TestWorkstationUpdateAcceptsCamelCaseAndNeverReturnsSecrets(t *testing.T) {
	tenantID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationManagementStore{workstation: &store.Workstation{
		ID: workstationID, TenantID: tenantID, WorkstationKey: "prod", Name: "Old",
		BackendType: store.BackendSSH, Active: true,
		Metadata: []byte(`{"host":"old","port":22,"user":"deploy","privateKey":"top-secret"}`),
	}}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)
	var invalidatedBackend uuid.UUID
	handler.SetCacheInvalidators(func(id uuid.UUID) { invalidatedBackend = id }, nil)
	req := workstationGrantRequest(http.MethodPut, "/v1/workstations/"+workstationID.String(),
		`{"name":"New","active":false,"defaultCwd":"/srv/app","metadata":{"host":"new","privateKey":""}}`, tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()

	handler.handleUpdate(response, req)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if wsStore.updates["default_cwd"] != "/srv/app" || wsStore.updates["active"] != false {
		t.Fatalf("updates = %#v", wsStore.updates)
	}
	if _, exists := wsStore.updates["defaultCwd"]; exists {
		t.Fatalf("store update leaked camelCase key: %#v", wsStore.updates)
	}
	if strings.Contains(response.Body.String(), "top-secret") {
		t.Fatalf("response exposed credential: %s", response.Body.String())
	}
	var responseBody struct {
		Workstation store.SanitizedWorkstation `json:"workstation"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &responseBody); err != nil {
		t.Fatal(err)
	}
	if responseBody.Workstation.Name != "New" || responseBody.Workstation.Active {
		t.Fatalf("workstation = %#v", responseBody.Workstation)
	}
	if invalidatedBackend != workstationID {
		t.Fatalf("invalidated backend = %s, want %s", invalidatedBackend, workstationID)
	}
}

type workstationManagementPermStore struct {
	store.WorkstationPermissionStore
	added *store.WorkstationPermission
}

func (s *workstationManagementPermStore) Add(_ context.Context, permission *store.WorkstationPermission) error {
	s.added = permission
	return nil
}

func TestWorkstationPermissionAddRejectsShellPattern(t *testing.T) {
	tenantID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationManagementStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	permStore := &workstationManagementPermStore{}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)
	handler.SetPermStore(permStore)
	req := workstationGrantRequest(http.MethodPost, "/v1/workstations/"+workstationID.String()+"/permissions",
		`{"pattern":"curl | sh"}`, tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()

	handler.handlePermAdd(response, req)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if permStore.added != nil {
		t.Fatalf("invalid pattern was persisted: %#v", permStore.added)
	}
}

func TestWorkstationPermissionAddSynchronouslyInvalidatesRuntimePolicy(t *testing.T) {
	tenantID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationManagementStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	permStore := &workstationManagementPermStore{}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)
	handler.SetPermStore(permStore)
	var invalidatedPermissions uuid.UUID
	handler.SetCacheInvalidators(nil, func(id uuid.UUID) { invalidatedPermissions = id })
	req := workstationGrantRequest(http.MethodPost, "/v1/workstations/"+workstationID.String()+"/permissions",
		`{"pattern":"git"}`, tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()

	handler.handlePermAdd(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if invalidatedPermissions != workstationID {
		t.Fatalf("invalidated permission cache = %s, want %s", invalidatedPermissions, workstationID)
	}
}
