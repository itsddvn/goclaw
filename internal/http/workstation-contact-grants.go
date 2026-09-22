package http

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

func (h *WorkstationsHandler) requireContactGrantStore(w http.ResponseWriter, locale string) bool {
	if h.contactGrantStore != nil {
		return true
	}
	writeError(w, http.StatusNotImplemented, protocol.ErrNotImplemented,
		i18n.T(locale, i18n.MsgNotImplemented, "workstation Contact grants"))
	return false
}

func (h *WorkstationsHandler) handleContactGrantList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requireContactGrantStore(w, locale) {
		return
	}
	workstationID, ok := h.requireContactGrantWorkstation(w, r, locale)
	if !ok {
		return
	}
	grants, err := h.contactGrantStore.ListForWorkstation(ctx, workstationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "workstation Contact grants"))
		return
	}
	if grants == nil {
		grants = []store.WorkstationContactGrant{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"grants": grants})
}

func (h *WorkstationsHandler) handleContactGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requireContactGrantStore(w, locale) {
		return
	}
	workstationID, ok := h.requireContactGrantWorkstation(w, r, locale)
	if !ok {
		return
	}
	var body struct {
		ContactID string `json:"contactId"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if !bindJSON(w, r, locale, &body) {
		return
	}
	contactID, err := uuid.Parse(body.ContactID)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "contact"))
		return
	}
	grant := &store.WorkstationContactGrant{
		WorkstationID: workstationID,
		ContactID:     contactID,
		CreatedBy:     store.ActorIDFromContext(ctx),
	}
	if err := h.contactGrantStore.Grant(ctx, grant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationContactGrantTargetNotFound))
			return
		}
		slog.Error("workstation.contact_grant_failed", "workstation_id", workstationID, "contact_id", contactID, "error", err)
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToCreate, "workstation Contact grant", err.Error()))
		return
	}
	slog.Info("workstation.contact_granted", "workstation_id", workstationID, "contact_id", contactID, "granted_by", grant.CreatedBy)
	writeJSON(w, http.StatusCreated, map[string]any{"grant": grant})
}

func (h *WorkstationsHandler) handleContactRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requireContactGrantStore(w, locale) {
		return
	}
	workstationID, ok := h.requireContactGrantWorkstation(w, r, locale)
	if !ok {
		return
	}
	contactID, err := uuid.Parse(r.PathValue("contactID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "contact"))
		return
	}
	if err := h.contactGrantStore.Revoke(ctx, workstationID, contactID); err != nil {
		slog.Error("workstation.contact_revoke_failed", "workstation_id", workstationID, "contact_id", contactID, "error", err)
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToDelete, "workstation Contact grant", err.Error()))
		return
	}
	slog.Info("workstation.contact_revoked", "workstation_id", workstationID, "contact_id", contactID, "revoked_by", store.ActorIDFromContext(ctx))
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *WorkstationsHandler) requireContactGrantWorkstation(w http.ResponseWriter, r *http.Request, locale string) (uuid.UUID, bool) {
	workstationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return uuid.Nil, false
	}
	if _, err := h.wsStore.GetByID(r.Context(), workstationID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, workstationID.String()))
			return uuid.Nil, false
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return uuid.Nil, false
	}
	return workstationID, true
}
