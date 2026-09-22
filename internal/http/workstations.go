package http

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/eventbus"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/workstation"
	workstationsecurity "github.com/nextlevelbuilder/goclaw/internal/workstation/security"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// WorkstationsHandler handles HTTP CRUD for workstations.
// Routes are only registered when edition is Standard — callers MUST gate.
type WorkstationsHandler struct {
	wsStore               store.WorkstationStore
	linkStore             store.AgentWorkstationLinkStore
	tenantStore           store.TenantStore
	permStore             store.WorkstationPermissionStore // optional; endpoints fail closed when absent
	activityStore         store.WorkstationActivityStore   // optional activity audit store
	contactGrantStore     store.WorkstationContactGrantStore
	eventBus              eventbus.DomainEventBus
	invalidateBackend     func(uuid.UUID)
	invalidatePermissions func(uuid.UUID)
}

// NewWorkstationsHandler creates a WorkstationsHandler.
func NewWorkstationsHandler(
	wsStore store.WorkstationStore,
	linkStore store.AgentWorkstationLinkStore,
	tenantStore store.TenantStore,
) *WorkstationsHandler {
	return &WorkstationsHandler{wsStore: wsStore, linkStore: linkStore, tenantStore: tenantStore}
}

// SetPermStore wires the permission store for allowlist CRUD endpoints.
func (h *WorkstationsHandler) SetPermStore(ps store.WorkstationPermissionStore) {
	h.permStore = ps
}

// SetActivityStore wires the activity store for audit log endpoints.
func (h *WorkstationsHandler) SetActivityStore(as store.WorkstationActivityStore) {
	h.activityStore = as
}

// SetContactGrantStore wires exact Contact authorization management.
func (h *WorkstationsHandler) SetContactGrantStore(s store.WorkstationContactGrantStore) {
	h.contactGrantStore = s
}

// SetEventBus wires lifecycle event publishing for runtime cache invalidation.
func (h *WorkstationsHandler) SetEventBus(bus eventbus.DomainEventBus) {
	h.eventBus = bus
}

// SetCacheInvalidators wires the runtime caches that must be synchronously
// evicted before a successful admin mutation is acknowledged.
func (h *WorkstationsHandler) SetCacheInvalidators(
	backend func(uuid.UUID),
	permissions func(uuid.UUID),
) {
	h.invalidateBackend = backend
	h.invalidatePermissions = permissions
}

// RegisterRoutes registers all workstation endpoints onto mux.
// MUST only be called after edition gate check — never in Lite builds.
func (h *WorkstationsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/workstations", h.auth(h.handleList))
	mux.HandleFunc("POST /v1/workstations", h.auth(h.handleCreate))
	mux.HandleFunc("GET /v1/workstations/{id}", h.auth(h.handleGet))
	mux.HandleFunc("PUT /v1/workstations/{id}", h.auth(h.handleUpdate))
	mux.HandleFunc("DELETE /v1/workstations/{id}", h.auth(h.handleDelete))
	mux.HandleFunc("POST /v1/workstations/{id}/test", h.auth(h.handleTest))
	// Agent assignments: strict default-deny access control for workstation execution.
	mux.HandleFunc("GET /v1/workstations/{id}/grants", h.auth(h.handleAgentGrantList))
	mux.HandleFunc("POST /v1/workstations/{id}/grants/agent", h.auth(h.handleAgentGrant))
	mux.HandleFunc("DELETE /v1/workstations/{id}/grants/agent/{agentID}", h.auth(h.handleAgentRevoke))
	// Exact Contacts allowed to instruct an assigned Agent to use this workstation.
	mux.HandleFunc("GET /v1/workstations/{id}/contact-grants", h.auth(h.handleContactGrantList))
	mux.HandleFunc("POST /v1/workstations/{id}/contact-grants", h.auth(h.handleContactGrant))
	mux.HandleFunc("DELETE /v1/workstations/{id}/contact-grants/{contactID}", h.auth(h.handleContactRevoke))
	// Permission allowlist CRUD.
	mux.HandleFunc("GET /v1/workstations/{id}/permissions", h.auth(h.handlePermList))
	mux.HandleFunc("POST /v1/workstations/{id}/permissions", h.auth(h.handlePermAdd))
	mux.HandleFunc("DELETE /v1/workstations/{id}/permissions/{permId}", h.auth(h.handlePermRemove))
	mux.HandleFunc("PUT /v1/workstations/{id}/permissions/{permId}/toggle", h.auth(h.handlePermToggle))
	// Activity audit log.
	mux.HandleFunc("GET /v1/workstations/{id}/activity", h.auth(h.handleActivityList))
}

func (h *WorkstationsHandler) auth(next http.HandlerFunc) http.HandlerFunc {
	return requireAuth(permissions.RoleAdmin, next)
}

func (h *WorkstationsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	wss, err := h.wsStore.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "workstations"))
		return
	}
	views := make([]*store.SanitizedWorkstation, len(wss))
	for i := range wss {
		views[i] = wss[i].SanitizedView()
	}
	writeJSON(w, http.StatusOK, map[string]any{"workstations": views})
}

func (h *WorkstationsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	ws, err := h.wsStore.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, idStr))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workstation": ws.SanitizedView()})
}

func (h *WorkstationsHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}

	var body struct {
		WorkstationKey string                   `json:"workstationKey"`
		Name           string                   `json:"name"`
		BackendType    store.WorkstationBackend `json:"backendType"`
		Metadata       json.RawMessage          `json:"metadata"`
		DefaultCWD     string                   `json:"defaultCwd"`
		DefaultEnv     json.RawMessage          `json:"defaultEnv"`
	}
	if !bindJSON(w, r, locale, &body) {
		return
	}

	if body.WorkstationKey == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgRequired, "workstationKey"))
		return
	}
	if !workstation.ValidateWorkstationKey(body.WorkstationKey) {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidSlug, "workstationKey"))
		return
	}
	if !workstation.ValidateBackend(body.BackendType) {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidBackend, string(body.BackendType)))
		return
	}
	metaBytes := []byte(body.Metadata)
	if err := store.ValidateMetadata(body.BackendType, metaBytes); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidMetadataShape, string(body.BackendType), err.Error()))
		return
	}
	envBytes := []byte(body.DefaultEnv)
	if len(envBytes) == 0 {
		envBytes = []byte("{}")
	}

	userID := store.UserIDFromContext(ctx)
	ws := &store.Workstation{
		WorkstationKey: body.WorkstationKey,
		Name:           body.Name,
		BackendType:    body.BackendType,
		Metadata:       metaBytes,
		DefaultCWD:     body.DefaultCWD,
		DefaultEnv:     envBytes,
		Active:         true,
		CreatedBy:      userID,
	}
	if err := h.wsStore.Create(ctx, ws); err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToCreate, "workstation", err.Error()))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"workstation": ws.SanitizedView()})
}

func (h *WorkstationsHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	var body workstation.AdminUpdate
	if !bindJSON(w, r, locale, &body) {
		return
	}
	current, err := h.wsStore.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, idStr))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	updates, err := workstation.BuildAdminUpdates(current, body)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidRequest, err.Error()))
		return
	}
	if err := h.wsStore.Update(ctx, id, updates); err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToUpdate, "workstation", err.Error()))
		return
	}
	if h.invalidateBackend != nil {
		h.invalidateBackend(id)
	}
	h.publishWorkstationEvent(ctx, eventbus.EventWorkstationUpdated, id)
	updated, err := h.wsStore.GetByID(ctx, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"workstation": updated.SanitizedView()})
}

func (h *WorkstationsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	if err := h.wsStore.Delete(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToDelete, "workstation", err.Error()))
		return
	}
	if h.invalidateBackend != nil {
		h.invalidateBackend(id)
	}
	h.publishWorkstationEvent(ctx, eventbus.EventWorkstationDeleted, id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id})
}

func (h *WorkstationsHandler) handleTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	ws, err := h.wsStore.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, id.String()))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	backend, err := workstation.Open(ws)
	if err != nil {
		writeError(w, http.StatusBadGateway, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgWorkstationConnectionFailed, err.Error()))
		return
	}
	defer func() { _ = backend.Close() }()
	testCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err := backend.HealthCheck(testCtx); err != nil {
		writeError(w, http.StatusBadGateway, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgWorkstationConnectionFailed, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// --- Workstation permission allowlist CRUD ---

func (h *WorkstationsHandler) requirePermStore(w http.ResponseWriter, locale string) bool {
	if h.permStore == nil {
		writeError(w, http.StatusNotImplemented, protocol.ErrNotImplemented,
			i18n.T(locale, i18n.MsgNotImplemented, "workstations permissions"))
		return false
	}
	return true
}

func (h *WorkstationsHandler) handlePermList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requirePermStore(w, locale) {
		return
	}
	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	// Ownership check: verify workstation belongs to caller's tenant before listing perms.
	// GetByID scopes the query by tenant_id — returns ErrNoRows for a different tenant.
	if _, err := h.wsStore.GetByID(ctx, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, wsID.String()))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	perms, err := h.permStore.ListForWorkstation(ctx, wsID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "permissions"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permissions": perms})
}

func (h *WorkstationsHandler) handlePermAdd(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requirePermStore(w, locale) {
		return
	}
	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}
	// Verify workstation belongs to caller's tenant before adding permission.
	// GetByID scopes the query by tenant_id in the WHERE clause — returns ErrNoRows if
	// the workstation exists in a different tenant.
	if _, err := h.wsStore.GetByID(ctx, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, wsID.String()))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}
	var body struct {
		Pattern string `json:"pattern"`
	}
	if !bindJSON(w, r, locale, &body) {
		return
	}
	if body.Pattern == "" {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgRequired, "pattern"))
		return
	}
	if err := workstationsecurity.ValidateAllowedBinaryPattern(body.Pattern); err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidRequest, err.Error()))
		return
	}
	userID := store.UserIDFromContext(ctx)
	perm := &store.WorkstationPermission{
		WorkstationID: wsID,
		Pattern:       body.Pattern,
		Enabled:       true,
		CreatedBy:     userID,
	}
	if err := h.permStore.Add(ctx, perm); err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToCreate, "permission", err.Error()))
		return
	}
	if h.invalidatePermissions != nil {
		h.invalidatePermissions(wsID)
	}
	h.publishWorkstationEvent(ctx, eventbus.EventWorkstationPermChanged, wsID)
	writeJSON(w, http.StatusCreated, map[string]any{"permission": perm})
}

func (h *WorkstationsHandler) handlePermRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requirePermStore(w, locale) {
		return
	}
	wsID, permID, ok := h.parseWorkstationPermissionPath(w, r, locale)
	if !ok {
		return
	}
	if !h.requirePermissionForWorkstation(w, r, locale, wsID, permID) {
		return
	}
	if err := h.permStore.Remove(ctx, permID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationPermNotFound, permID.String()))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToDelete, "permission", err.Error()))
		return
	}
	if h.invalidatePermissions != nil {
		h.invalidatePermissions(wsID)
	}
	h.publishWorkstationEvent(ctx, eventbus.EventWorkstationPermChanged, wsID)
	writeJSON(w, http.StatusOK, map[string]any{"id": permID})
}

func (h *WorkstationsHandler) handlePermToggle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) || !h.requirePermStore(w, locale) {
		return
	}
	wsID, permID, ok := h.parseWorkstationPermissionPath(w, r, locale)
	if !ok {
		return
	}
	if !h.requirePermissionForWorkstation(w, r, locale, wsID, permID) {
		return
	}
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if !bindJSON(w, r, locale, &body) {
		return
	}
	if err := h.permStore.SetEnabled(ctx, permID, body.Enabled); err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToUpdate, "permission", err.Error()))
		return
	}
	if h.invalidatePermissions != nil {
		h.invalidatePermissions(wsID)
	}
	h.publishWorkstationEvent(ctx, eventbus.EventWorkstationPermChanged, wsID)
	writeJSON(w, http.StatusOK, map[string]any{"id": permID, "enabled": body.Enabled})
}

// --- Workstation activity audit log ---

func (h *WorkstationsHandler) handleActivityList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	locale := store.LocaleFromContext(ctx)
	if !requireTenantAdmin(w, r, h.tenantStore) {
		return
	}
	if h.activityStore == nil {
		writeError(w, http.StatusNotImplemented, protocol.ErrNotImplemented,
			i18n.T(locale, i18n.MsgNotImplemented, "workstations activity"))
		return
	}
	wsID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, protocol.ErrInvalidRequest,
			i18n.T(locale, i18n.MsgInvalidID, "workstation"))
		return
	}

	// Ownership check: verify the workstation belongs to the caller's tenant.
	// GetByID scopes by tenant_id — returns ErrNoRows if workstation is in a different tenant.
	if _, err := h.wsStore.GetByID(ctx, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, protocol.ErrNotFound,
				i18n.T(locale, i18n.MsgWorkstationNotFound, wsID.String()))
			return
		}
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgInternalError, err.Error()))
		return
	}

	limit := 50
	if lStr := r.URL.Query().Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}
	var cursor *uuid.UUID
	if cStr := r.URL.Query().Get("cursor"); cStr != "" {
		if cID, err := uuid.Parse(cStr); err == nil {
			cursor = &cID
		}
	}

	rows, nextCursor, err := h.activityStore.List(ctx, wsID, limit, cursor)
	if err != nil {
		writeError(w, http.StatusInternalServerError, protocol.ErrInternal,
			i18n.T(locale, i18n.MsgFailedToList, "activity"))
		return
	}

	resp := map[string]any{"activity": rows}
	if nextCursor != nil {
		resp["nextCursor"] = nextCursor.String()
	}
	writeJSON(w, http.StatusOK, resp)
}
