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

type workstationContactGrantStore struct {
	store.WorkstationContactGrantStore
	grants  []store.WorkstationContactGrant
	granted *store.WorkstationContactGrant
	revoked [2]uuid.UUID
}

func (s *workstationContactGrantStore) Grant(_ context.Context, grant *store.WorkstationContactGrant) error {
	copy := *grant
	copy.ChannelType = "telegram"
	copy.SenderID = "contact-a"
	s.granted = &copy
	*grant = copy
	return nil
}

func (s *workstationContactGrantStore) Revoke(_ context.Context, workstationID, contactID uuid.UUID) error {
	s.revoked = [2]uuid.UUID{workstationID, contactID}
	return nil
}

func (s *workstationContactGrantStore) ListForWorkstation(_ context.Context, _ uuid.UUID) ([]store.WorkstationContactGrant, error) {
	return s.grants, nil
}

func TestWorkstationContactGrantHandlersUseContactUUIDContract(t *testing.T) {
	tenantID, workstationID, contactID := uuid.New(), uuid.New(), uuid.New()
	wsStore := &workstationGrantWorkstationStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	grantStore := &workstationContactGrantStore{}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)
	handler.SetContactGrantStore(grantStore)

	grantReq := workstationGrantRequest(http.MethodPost, "/v1/workstations/"+workstationID.String()+"/contact-grants",
		`{"contactId":"`+contactID.String()+`"}`, tenantID)
	grantReq.SetPathValue("id", workstationID.String())
	grantResponse := httptest.NewRecorder()
	handler.handleContactGrant(grantResponse, grantReq)
	if grantResponse.Code != http.StatusCreated {
		t.Fatalf("grant status = %d, body = %s", grantResponse.Code, grantResponse.Body.String())
	}
	if grantStore.granted == nil || grantStore.granted.WorkstationID != workstationID || grantStore.granted.ContactID != contactID {
		t.Fatalf("grant = %#v", grantStore.granted)
	}

	revokeReq := workstationGrantRequest(http.MethodDelete, "/v1/workstations/"+workstationID.String()+"/contact-grants/"+contactID.String(), "", tenantID)
	revokeReq.SetPathValue("id", workstationID.String())
	revokeReq.SetPathValue("contactID", contactID.String())
	revokeResponse := httptest.NewRecorder()
	handler.handleContactRevoke(revokeResponse, revokeReq)
	if revokeResponse.Code != http.StatusOK || grantStore.revoked != [2]uuid.UUID{workstationID, contactID} {
		t.Fatalf("revoke status = %d, revoked = %#v", revokeResponse.Code, grantStore.revoked)
	}
}

func TestWorkstationContactGrantListReturnsEmptyArray(t *testing.T) {
	tenantID, workstationID := uuid.New(), uuid.New()
	wsStore := &workstationGrantWorkstationStore{workstation: &store.Workstation{ID: workstationID, TenantID: tenantID}}
	handler := NewWorkstationsHandler(wsStore, &workstationGrantLinkStore{}, nil)
	handler.SetContactGrantStore(&workstationContactGrantStore{})
	req := workstationGrantRequest(http.MethodGet, "/v1/workstations/"+workstationID.String()+"/contact-grants", "", tenantID)
	req.SetPathValue("id", workstationID.String())
	response := httptest.NewRecorder()

	handler.handleContactGrantList(response, req)

	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"grants":[]`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}
