package routers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type ToolsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type ToolsRouter struct {
	Config ToolsRuntimeConfig
	Users  *models.UsersTable
	Tools  *models.ToolsTable
}

type toolForm struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Content string         `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func (h *ToolsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/tools/", h.GetTools)
	mux.HandleFunc("GET /api/v1/tools/list", h.GetToolList)
	mux.HandleFunc("GET /api/v1/tools/export", h.ExportTools)
	mux.HandleFunc("POST /api/v1/tools/create", h.CreateTool)
	mux.HandleFunc("GET /api/v1/tools/id/{id}", h.GetToolByID)
	mux.HandleFunc("POST /api/v1/tools/id/{id}/update", h.UpdateToolByID)
	mux.HandleFunc("DELETE /api/v1/tools/id/{id}/delete", h.DeleteToolByID)
}

func (h *ToolsRouter) GetTools(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if user.Role == "admin" {
		tools, err := h.Tools.GetTools(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, tools)
		return
	}
	tools, err := h.Tools.GetToolsByUserID(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tools)
}

func (h *ToolsRouter) GetToolList(w http.ResponseWriter, r *http.Request) {
	h.GetTools(w, r)
}

func (h *ToolsRouter) ExportTools(w http.ResponseWriter, r *http.Request) {
	h.GetTools(w, r)
}

func (h *ToolsRouter) CreateTool(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form toolForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	matched, _ := regexp.MatchString(`^[A-Za-z_][A-Za-z0-9_]*$`, form.ID)
	if !matched {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "Only alphanumeric characters and underscores are allowed in the id"})
		return
	}
	form.ID = strings.ToLower(form.ID)
	existing, err := h.Tools.GetToolByID(r.Context(), form.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if existing != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "id taken"})
		return
	}
	tool, err := h.Tools.InsertNewTool(r.Context(), models.ToolCreateParams{
		ID:      form.ID,
		UserID:  user.ID,
		Name:    form.Name,
		Content: form.Content,
		Meta:    form.Meta,
		Specs:   []map[string]any{},
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

func (h *ToolsRouter) GetToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	tool, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if tool == nil || (user.Role != "admin" && tool.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

func (h *ToolsRouter) UpdateToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || (user.Role != "admin" && current.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	var form toolForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	tool, err := h.Tools.UpdateToolByID(r.Context(), current.ID, models.ToolUpdateParams{
		Name:    toolStringPtr(form.Name),
		Content: toolStringPtr(form.Content),
		Meta:    form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, tool)
}

func (h *ToolsRouter) DeleteToolByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Tools.GetToolByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || (user.Role != "admin" && current.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Tools.DeleteToolByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *ToolsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	if strings.HasPrefix(token, "sk-") {
		if !h.Config.EnableAPIKeys {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "api key not allowed"})
			return nil, false
		}
		user, err := h.Users.GetUserByAPIKey(r.Context(), token)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return nil, false
		}
		if user != nil {
			return user, true
		}
	} else {
		claims, err := utils.DecodeToken(h.Config.WebUISecretKey, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
			return nil, false
		}
		user, err := h.Users.GetUserByID(r.Context(), claims.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return nil, false
		}
		if user != nil {
			return user, true
		}
	}
	writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
	return nil, false
}

func toolStringPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
