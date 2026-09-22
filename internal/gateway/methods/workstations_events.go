package methods

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func (m *WorkstationsMethods) publishWorkstationEvent(ctx context.Context, eventType eventbus.EventType, id uuid.UUID) {
	if m.eventBus == nil {
		return
	}
	m.eventBus.Publish(eventbus.DomainEvent{
		ID:       uuid.New().String(),
		Type:     eventType,
		TenantID: store.TenantIDFromContext(ctx).String(),
		Payload:  map[string]any{"workstation_id": id.String()},
	})
}

func (m *WorkstationsMethods) requirePermissionForWorkstation(
	ctx context.Context,
	client *gateway.Client,
	req *protocol.RequestFrame,
	workstationID uuid.UUID,
	permissionID uuid.UUID,
) bool {
	locale := store.LocaleFromContext(ctx)
	if _, err := m.wsStore.GetByID(ctx, workstationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, workstationID.String())))
			return false
		}
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error())))
		return false
	}
	permissions, err := m.permStore.ListForWorkstation(ctx, workstationID)
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "permissions")))
		return false
	}
	for _, permission := range permissions {
		if permission.ID == permissionID {
			return true
		}
	}
	client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrNotFound,
		i18n.T(locale, i18n.MsgWorkstationPermNotFound, permissionID.String())))
	return false
}
