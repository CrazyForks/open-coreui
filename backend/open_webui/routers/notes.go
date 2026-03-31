package routers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type NotesRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type NotesRouter struct {
	Config NotesRuntimeConfig
	Users  *models.UsersTable
	Notes  *models.NotesTable
}

type noteForm struct {
	Title string         `json:"title"`
	Data  map[string]any `json:"data,omitempty"`
	Meta  map[string]any `json:"meta,omitempty"`
}

type noteListResponse struct {
	Items []models.Note `json:"items"`
	Total int           `json:"total"`
}

func (h *NotesRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notes/", h.GetNotes)
	mux.HandleFunc("GET /api/v1/notes/search", h.SearchNotes)
	mux.HandleFunc("POST /api/v1/notes/create", h.CreateNote)
	mux.HandleFunc("GET /api/v1/notes/{id}", h.GetNoteByID)
	mux.HandleFunc("POST /api/v1/notes/{id}/update", h.UpdateNoteByID)
	mux.HandleFunc("DELETE /api/v1/notes/{id}", h.DeleteNoteByID)
}

func (h *NotesRouter) GetNotes(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 0)
	limit := 0
	skip := 0
	if page > 0 {
		limit = 60
		skip = (page - 1) * limit
	}
	notes, err := h.Notes.GetNotesByUserID(r.Context(), user.ID, skip, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, notes)
}

func (h *NotesRouter) SearchNotes(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 60
	skip := (page - 1) * limit
	notes, total, err := h.Notes.SearchNotes(r.Context(), user.ID, models.NoteListOptions{
		Query: r.URL.Query().Get("query"),
		Skip:  skip,
		Limit: limit,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, noteListResponse{Items: notes, Total: total})
}

func (h *NotesRouter) CreateNote(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	var form noteForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	note, err := h.Notes.InsertNewNote(r.Context(), models.NoteCreateParams{
		UserID: user.ID,
		Title:  form.Title,
		Data:   form.Data,
		Meta:   form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *NotesRouter) GetNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	note, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if note == nil || (user.Role != "admin" && note.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *NotesRouter) UpdateNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || (user.Role != "admin" && current.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}

	var form noteForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	note, err := h.Notes.UpdateNoteByID(r.Context(), current.ID, models.NoteUpdateParams{
		Title: noteStringPtr(form.Title),
		Data:  form.Data,
		Meta:  form.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, note)
}

func (h *NotesRouter) DeleteNoteByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	current, err := h.Notes.GetNoteByID(r.Context(), r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if current == nil || (user.Role != "admin" && current.UserID != user.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	deleted, err := h.Notes.DeleteNoteByID(r.Context(), current.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *NotesRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func noteStringPtr(value string) *string {
	return &value
}
