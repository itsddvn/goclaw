package methods

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func (m *WorkstationsMethods) requireContactGrantStore(locale string, client *gateway.Client, req *protocol.RequestFrame) bool {
	if m.contactGrantStore != nil {
		return true
	}
	client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotImplemented,
		i18n.T(locale, i18n.MsgNotImplemented, "workstation Contact grants")))
	return false
}

func (m *WorkstationsMethods) handleContactGrantList(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	if !m.requireContactGrantStore(locale, client, req) {
		return
	}
	var params struct {
		WorkstationID string `json:"workstationId"`
	}
	if req.Params != nil && json.Unmarshal(req.Params, &params) != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	workstationID, ok := m.requireContactGrantWorkstation(ctx, client, req, params.WorkstationID)
	if !ok {
		return
	}
	grants, err := m.contactGrantStore.ListForWorkstation(ctx, workstationID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "workstation Contact grants")))
		return
	}
	if grants == nil {
		grants = []store.WorkstationContactGrant{}
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"grants": grants}))
}

func (m *WorkstationsMethods) handleContactGrant(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	if !m.requireContactGrantStore(locale, client, req) {
		return
	}
	var params struct {
		WorkstationID string `json:"workstationId"`
		ContactID     string `json:"contactId"`
	}
	if req.Params != nil && json.Unmarshal(req.Params, &params) != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	workstationID, ok := m.requireContactGrantWorkstation(ctx, client, req, params.WorkstationID)
	if !ok {
		return
	}
	contactID, err := uuid.Parse(params.ContactID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "contact")))
		return
	}
	grant := &store.WorkstationContactGrant{
		WorkstationID: workstationID,
		ContactID:     contactID,
		CreatedBy:     client.UserID(),
	}
	if err := m.contactGrantStore.Grant(ctx, grant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationContactGrantTargetNotFound)))
			return
		}
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToCreate, "workstation Contact grant", err.Error())))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"grant": grant}))
}

func (m *WorkstationsMethods) handleContactRevoke(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	if !m.requireContactGrantStore(locale, client, req) {
		return
	}
	var params struct {
		WorkstationID string `json:"workstationId"`
		ContactID     string `json:"contactId"`
	}
	if req.Params != nil && json.Unmarshal(req.Params, &params) != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest, "invalid params"))
		return
	}
	workstationID, ok := m.requireContactGrantWorkstation(ctx, client, req, params.WorkstationID)
	if !ok {
		return
	}
	contactID, err := uuid.Parse(params.ContactID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "contact")))
		return
	}
	if err := m.contactGrantStore.Revoke(ctx, workstationID, contactID); err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToDelete, "workstation Contact grant", err.Error())))
		return
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{"revoked": true}))
}

func (m *WorkstationsMethods) requireContactGrantWorkstation(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame, rawID string) (uuid.UUID, bool) {
	locale := store.LocaleFromContext(ctx)
	workstationID, err := uuid.Parse(rawID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation")))
		return uuid.Nil, false
	}
	if _, err := m.wsStore.GetByID(ctx, workstationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, rawID)))
			return uuid.Nil, false
		}
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error())))
		return uuid.Nil, false
	}
	return workstationID, true
}
