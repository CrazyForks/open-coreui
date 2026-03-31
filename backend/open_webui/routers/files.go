package routers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/storage"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type FilesRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
}

type FilesRouter struct {
	Config  FilesRuntimeConfig
	Users   *models.UsersTable
	Files   *models.FilesTable
	Storage storage.Provider
}

type fileListResponse struct {
	Items []models.File `json:"items"`
	Total int           `json:"total"`
}

func (h *FilesRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/files/", h.UploadFile)
	mux.HandleFunc("GET /api/v1/files/", h.GetFiles)
	mux.HandleFunc("GET /api/v1/files/search", h.SearchFiles)
	mux.HandleFunc("GET /api/v1/files/{id}", h.GetFileByID)
	mux.HandleFunc("GET /api/v1/files/{id}/content", h.GetFileContentByID)
	mux.HandleFunc("DELETE /api/v1/files/{id}", h.DeleteFileByID)
	mux.HandleFunc("DELETE /api/v1/files/all", h.DeleteAllFiles)
}

func (h *FilesRouter) UploadFile(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid multipart body"})
		return
	}
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "missing file"})
		return
	}
	defer file.Close()

	metadata := map[string]any{}
	if raw := r.FormValue("metadata"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &metadata)
	}

	id := uuid.NewString()
	storedFilename := id + "_" + filepath.Base(fileHeader.Filename)
	contents, filePath, err := h.Storage.UploadFile(file, storedFilename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	fileModel, err := h.Files.InsertNewFile(r.Context(), models.FileCreateParams{
		ID:       id,
		UserID:   user.ID,
		Filename: fileHeader.Filename,
		Path:     filePath,
		Data:     map[string]any{},
		Meta: map[string]any{
			"name":         fileHeader.Filename,
			"content_type": fileHeader.Header.Get("Content-Type"),
			"size":         len(contents),
			"data":         metadata,
		},
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, fileModel)
}

func (h *FilesRouter) GetFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	page := parseIntQuery(r, "page", 1)
	if page < 1 {
		page = 1
	}
	limit := 50
	skip := (page - 1) * limit
	items, total, err := h.Files.GetFileList(r.Context(), models.FileListOptions{UserID: user.ID, Skip: skip, Limit: limit})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, fileListResponse{Items: items, Total: total})
}

func (h *FilesRouter) SearchFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	items, _, err := h.Files.GetFileList(r.Context(), models.FileListOptions{UserID: user.ID, Query: r.URL.Query().Get("query"), Limit: 100})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (h *FilesRouter) GetFileByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, file)
}

func (h *FilesRouter) GetFileContentByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	localPath, err := h.Storage.GetFile(file.Path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	http.ServeFile(w, r, localPath)
}

func (h *FilesRouter) DeleteFileByID(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	file, err := h.Files.GetFileByIDAndUserID(r.Context(), r.PathValue("id"), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if file == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"detail": "not found"})
		return
	}
	if err := h.Storage.DeleteFile(file.Path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	deleted, err := h.Files.DeleteFileByID(r.Context(), file.ID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, deleted)
}

func (h *FilesRouter) DeleteAllFiles(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	items, _, err := h.Files.GetFileList(r.Context(), models.FileListOptions{UserID: user.ID, Limit: 10000})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	for _, file := range items {
		if err := h.Storage.DeleteFile(file.Path); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
		if _, err := h.Files.DeleteFileByID(r.Context(), file.ID); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *FilesRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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
