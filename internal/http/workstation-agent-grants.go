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

func (h *WorkstationsHandler) handleAgentGrantList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	workstationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	if _, err := h.wsStore.GetByID(ctx, workstationID); err != nil {
		h.writeWorkstationGrantError(w, locale, workstationID, err)
		return
	}
	grants, err := h.linkStore.ListForWorkstation(ctx, workstationID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "workstation grants"))
		return
	}
	if grants == nil {
		grants = []store.AgentWorkstationLink{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"grants": grants})
}

func (h *WorkstationsHandler) handleAgentGrant(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	workstationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	var body struct {
		AgentID   string `json:"agentId"`
		IsDefault bool   `json:"isDefault"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if !bindJSON(w, r, locale, &body) {
		return
	}
	agentID, err := uuid.Parse(body.AgentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "agent"))
		return
	}
	grant := &store.AgentWorkstationLink{
		AgentID:       agentID,
		WorkstationID: workstationID,
		IsDefault:     body.IsDefault,
	}
	if err := h.linkStore.Link(ctx, grant); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationGrantTargetNotFound))
			return
		}
		h.writeWorkstationGrantError(w, locale, workstationID, err)
		return
	}
	slog.Info("workstation.agent_granted",
		"workstation_id", workstationID,
		"agent_id", agentID,
		"is_default", body.IsDefault,
		"granted_by", store.UserIDFromContext(ctx),
	)
	writeJSON(w, http.StatusCreated, map[string]any{"grant": grant})
}

func (h *WorkstationsHandler) handleAgentRevoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	workstationID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	agentID, err := uuid.Parse(r.PathValue("agentID"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "agent"))
		return
	}
	if _, err := h.wsStore.GetByID(ctx, workstationID); err != nil {
		h.writeWorkstationGrantError(w, locale, workstationID, err)
		return
	}
	if err := h.linkStore.Unlink(ctx, agentID, workstationID); err != nil {
		slog.Error("workstation.agent_revoke_failed",
			"workstation_id", workstationID,
			"agent_id", agentID,
			"error", err,
		)
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, "workstation grant operation failed"))
		return
	}
	slog.Info("workstation.agent_revoked",
		"workstation_id", workstationID,
		"agent_id", agentID,
		"revoked_by", store.UserIDFromContext(ctx),
	)
	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *WorkstationsHandler) writeWorkstationGrantError(w http.ResponseWriter, locale string, workstationID uuid.UUID, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, http.StatusNotFound, protocol.ErrNotFound,
			i18n.T(locale, i18n.MsgWorkstationNotFound, workstationID.String()))
		return
	}
	slog.Error("workstation.agent_grant_failed",
		"workstation_id", workstationID,
		"error", err,
	)
	writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
		i18n.T(locale, i18n.MsgInternalError, "workstation grant operation failed"))
}
