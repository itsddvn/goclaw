package http

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
)

const maxSkillUploadSize = 20 << 20 // 20 MB

var slugRegexp = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// SkillsHandler handles skill management HTTP endpoints (managed mode).
type SkillsHandler struct {
	skills   *pg.PGSkillStore
	baseDir  string // filesystem base for skill content
	token    string
}

// NewSkillsHandler creates a handler for skill management endpoints.
func NewSkillsHandler(skills *pg.PGSkillStore, baseDir, token string) *SkillsHandler {
	return &SkillsHandler{skills: skills, baseDir: baseDir, token: token}
}

// RegisterRoutes registers all skill management routes on the given mux.
func (h *SkillsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/skills", h.authMiddleware(h.handleList))
	mux.HandleFunc("POST /v1/skills/upload", h.authMiddleware(h.handleUpload))
	mux.HandleFunc("GET /v1/skills/{id}", h.authMiddleware(h.handleGet))
	mux.HandleFunc("PUT /v1/skills/{id}", h.authMiddleware(h.handleUpdate))
	mux.HandleFunc("DELETE /v1/skills/{id}", h.authMiddleware(h.handleDelete))
	mux.HandleFunc("POST /v1/skills/{id}/grants/agent", h.authMiddleware(h.handleGrantAgent))
	mux.HandleFunc("DELETE /v1/skills/{id}/grants/agent/{agentID}", h.authMiddleware(h.handleRevokeAgent))
	mux.HandleFunc("POST /v1/skills/{id}/grants/user", h.authMiddleware(h.handleGrantUser))
	mux.HandleFunc("DELETE /v1/skills/{id}/grants/user/{userID}", h.authMiddleware(h.handleRevokeUser))
}

func (h *SkillsHandler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.token != "" {
			if extractBearerToken(r) != h.token {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				return
			}
		}
		userID := extractUserID(r)
		if userID != "" {
			ctx := store.WithUserID(r.Context(), userID)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

func (h *SkillsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	skills := h.skills.ListSkills()
	writeJSON(w, http.StatusOK, map[string]interface{}{"skills": skills})
}

func (h *SkillsHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	skill, ok := h.skills.GetSkill(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "skill not found"})
		return
	}
	writeJSON(w, http.StatusOK, skill)
}

func (h *SkillsHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	// Prevent changing sensitive fields
	delete(updates, "id")
	delete(updates, "owner_id")
	delete(updates, "file_path")

	if err := h.skills.UpdateSkill(id, updates); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *SkillsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	if err := h.skills.DeleteSkill(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// handleUpload processes a ZIP file upload containing a skill (must have SKILL.md at root).
func (h *SkillsHandler) handleUpload(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "X-GoClaw-User-Id header required"})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxSkillUploadSize)

	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file is required: " + err.Error()})
		return
	}
	defer file.Close()

	// Save to temp file for zip processing
	tmp, err := os.CreateTemp("", "skill-upload-*.zip")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create temp file"})
		return
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	hasher := sha256.New()
	size, err := io.Copy(io.MultiWriter(tmp, hasher), file)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save upload"})
		return
	}
	fileHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Open as zip
	zr, err := zip.OpenReader(tmp.Name())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ZIP file"})
		return
	}
	defer zr.Close()

	// Validate: must have SKILL.md at root
	var skillMD *zip.File
	for _, f := range zr.File {
		if f.Name == "SKILL.md" || f.Name == "./SKILL.md" {
			skillMD = f
			break
		}
	}
	if skillMD == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ZIP must contain SKILL.md at root"})
		return
	}

	// Read and parse SKILL.md frontmatter
	skillContent, err := readZipFile(skillMD)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read SKILL.md"})
		return
	}

	name, description, slug := parseSkillFrontmatter(skillContent)
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "SKILL.md must have a name in frontmatter"})
		return
	}
	if slug == "" {
		slug = slugify(name)
	}
	if !slugRegexp.MatchString(slug) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slug must be a valid slug (lowercase letters, numbers, hyphens only)"})
		return
	}

	// Determine version (increment if slug already exists)
	version := 1
	if existing, ok := h.skills.GetSkill(slug); ok {
		_ = existing
		version = h.skills.GetNextVersion(slug)
	}

	// Extract to filesystem: baseDir/slug/version/
	destDir := filepath.Join(h.baseDir, slug, fmt.Sprintf("%d", version))
	if err := os.MkdirAll(destDir, 0755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create skill directory"})
		return
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// Security: prevent path traversal
		name := filepath.Clean(f.Name)
		if strings.Contains(name, "..") {
			continue
		}
		destPath := filepath.Join(destDir, name)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			continue
		}
		data, err := readZipFile(f)
		if err != nil {
			continue
		}
		os.WriteFile(destPath, []byte(data), 0644)
	}

	// Save metadata to DB
	desc := description
	skill := pg.SkillCreateParams{
		Name:        name,
		Slug:        slug,
		Description: &desc,
		OwnerID:     userID,
		Visibility:  "private",
		Version:     version,
		FilePath:    destDir,
		FileSize:    size,
		FileHash:    &fileHash,
	}

	id, err := h.skills.CreateSkillManaged(r.Context(), skill)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save skill: " + err.Error()})
		return
	}

	h.skills.BumpVersion()
	slog.Info("skill uploaded", "id", id, "slug", slug, "version", version, "size", header.Size)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"id":      id,
		"slug":    slug,
		"version": version,
		"name":    name,
	})
}

func (h *SkillsHandler) handleGrantAgent(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	idStr := r.PathValue("id")
	skillID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
		Version int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent_id"})
		return
	}

	if req.Version <= 0 {
		req.Version = 1
	}

	if err := h.skills.GrantToAgent(r.Context(), skillID, agentID, req.Version, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (h *SkillsHandler) handleRevokeAgent(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	skillID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	agentIDStr := r.PathValue("agentID")
	agentID, err := uuid.Parse(agentIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent ID"})
		return
	}

	if err := h.skills.RevokeFromAgent(r.Context(), skillID, agentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

func (h *SkillsHandler) handleGrantUser(w http.ResponseWriter, r *http.Request) {
	userID := store.UserIDFromContext(r.Context())
	idStr := r.PathValue("id")
	skillID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	if err := store.ValidateUserID(req.UserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.skills.GrantToUser(r.Context(), skillID, req.UserID, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusCreated, map[string]string{"ok": "true"})
}

func (h *SkillsHandler) handleRevokeUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	skillID, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid skill ID"})
		return
	}

	targetUserID := r.PathValue("userID")
	if err := store.ValidateUserID(targetUserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.skills.RevokeFromUser(r.Context(), skillID, targetUserID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	h.skills.BumpVersion()
	writeJSON(w, http.StatusOK, map[string]string{"ok": "true"})
}

// --- Helpers ---

func readZipFile(f *zip.File) (string, error) {
	rc, err := f.Open()
	if err != nil {
		return "", err
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseSkillFrontmatter extracts name, description, and slug from SKILL.md YAML frontmatter.
func parseSkillFrontmatter(content string) (name, description, slug string) {
	if !strings.HasPrefix(content, "---") {
		return "", "", ""
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return "", "", ""
	}
	fm := content[3 : 3+end]

	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "name:") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			name = strings.Trim(name, `"'`)
		}
		if strings.HasPrefix(line, "description:") {
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			description = strings.Trim(description, `"'`)
		}
		if strings.HasPrefix(line, "slug:") {
			slug = strings.TrimSpace(strings.TrimPrefix(line, "slug:"))
			slug = strings.Trim(slug, `"'`)
		}
	}
	return
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, s)
	// Collapse multiple dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "skill"
	}
	return s
}
