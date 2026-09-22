package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type workstationGrantWorkstationStore struct {
	store.WorkstationStore
	workstation *store.Workstation
}

func (s *workstationGrantWorkstationStore) GetByID(_ context.Context, id uuid.UUID) (*store.Workstation, error) {
	if s.workstation != nil && s.workstation.ID == id {
		return s.workstation, nil
	}
	return nil, nil
}

type workstationGrantLinkStore struct {
	store.AgentWorkstationLinkStore
	links    []store.AgentWorkstationLink
	linked   *store.AgentWorkstationLink
	unlinked [2]uuid.UUID
}

func (s *workstationGrantLinkStore) Link(_ context.Context, link *store.AgentWorkstationLink) error {
	copy := *link
	s.linked = &copy
	return nil
}

func (s *workstationGrantLinkStore) Unlink(_ context.Context, agentID, workstationID uuid.UUID) error {
	s.unlinked = [2]uuid.UUID{agentID, workstationID}
	return nil
}

func (s *workstationGrantLinkStore) ListForWorkstation(_ context.Context, _ uuid.UUID) ([]store.AgentWorkstationLink, error) {
	return s.links, nil
}

func workstationGrantRequest(method, path, body string, tenantID uuid.UUID) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	ctx := store.WithTenantID(req.Context(), tenantID)
	ctx = store.WithRole(ctx, "owner")
	return req.WithContext(ctx)
}

func TestWorkstationAgentGrantHandlersUseCamelCaseContract(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationGrantWorkstationStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	linkStore := &workstationGrantLinkStore{}
	handler := NewWorkstationsHandler(wsStore, linkStore, nil)

	req := workstationGrantRequest(http.MethodPost, "/v1/workstations/"+workstationID.String()+"/grants/agent",
		`{"agentId":"`+agentID.String()+`","isDefault":true}`, tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()
	handler.handleAgentGrant(response, req)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if linkStore.linked == nil || linkStore.linked.AgentID != agentID || linkStore.linked.WorkstationID != workstationID {
		t.Fatalf("linked = %#v, want agent %s and workstation %s", linkStore.linked, agentID, workstationID)
	}
	if !linkStore.linked.IsDefault {
		t.Fatal("isDefault = false, want true")
	}
}

func TestWorkstationAgentGrantHandlersListAndRevoke(t *testing.T) {
	tenantID := uuid.New()
	agentID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationGrantWorkstationStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	linkStore := &workstationGrantLinkStore{links: []store.AgentWorkstationLink{{
		AgentID: agentID, WorkstationID: workstationID, TenantID: tenantID, IsDefault: true,
	}}}
	handler := NewWorkstationsHandler(wsStore, linkStore, nil)

	listReq := workstationGrantRequest(http.MethodGet, "/v1/workstations/"+workstationID.String()+"/grants", "", tenantID)
	listReq.SetPathValue("id", workstationID.String())
	listResponse := httptest.NewRecorder()
	handler.handleAgentGrantList(listResponse, listReq)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"isDefault":true`) {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}

	revokeReq := workstationGrantRequest(http.MethodDelete,
		"/v1/workstations/"+workstationID.String()+"/grants/agent/"+agentID.String(), "", tenantID)
	revokeReq.SetPathValue("id", workstationID.String())
	revokeReq.SetPathValue("agentID", agentID.String())
	revokeResponse := httptest.NewRecorder()
	handler.handleAgentRevoke(revokeResponse, revokeReq)
	if revokeResponse.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeResponse.Code, revokeResponse.Body.String())
	}
	if linkStore.unlinked != [2]uuid.UUID{agentID, workstationID} {
		t.Fatalf("unlinked = %#v", linkStore.unlinked)
	}
}

func TestWorkstationAgentGrantHandlersReturnEmptyArray(t *testing.T) {
	tenantID := uuid.New()
	workstationID := uuid.New()
	wsStore := &workstationGrantWorkstationStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)

	req := workstationGrantRequest(http.MethodGet, "/v1/workstations/"+workstationID.String()+"/grants", "", tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()
	handler.handleAgentGrantList(response, req)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"grants":[]`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
