package http

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func (h *WorkstationsHandler) publishWorkstationEvent(ctx context.Context, eventType eventbus.EventType, id uuid.UUID) {
	if h.eventBus == nil {
		return
	}
	h.eventBus.Publish(eventbus.DomainEvent{
		ID:       uuid.New().String(),
		Type:     eventType,
		// Mutation events must not use workstation ID as SourceID because the
		// domain bus deduplicates Type+SourceID for five minutes. Every mutation
		// needs to invalidate caches, so carry the target in the payload instead.
		TenantID: store.TenantIDFromContext(ctx).String(),
		Payload:  map[string]any{"workstation_id": id.String()},
	})
}

func (h *WorkstationsHandler) parseWorkstationPermissionPath(
	w http.ResponseWriter,
	r *http.Request,
	locale string,
) (uuid.UUID, uuid.UUID, bool) {
	workstationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return uuid.Nil, uuid.Nil, false
	}
	permissionID, err := uuid.Parse(r.PathValue("permId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "permission"))
		return uuid.Nil, uuid.Nil, false
	}
	return workstationID, permissionID, true
}

func (h *WorkstationsHandler) requirePermissionForWorkstation(
	w http.ResponseWriter,
	r *http.Request,
	locale string,
	workstationID uuid.UUID,
	permissionID uuid.UUID,
) bool {
	if _, err := h.wsStore.GetByID(r.Context(), workstationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, workstationID.String()))
			return false
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return false
	}
	permissions, err := h.permStore.ListForWorkstation(r.Context(), workstationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "permissions"))
		return false
	}
	for _, permission := range permissions {
		if permission.ID == permissionID {
			return true
		}
	}
	writeError(w, http.StatusNotFound, protocol.ErrNotFound,
		i18n.T(locale, i18n.MsgWorkstationPermNotFound, permissionID.String()))
	return false
}
