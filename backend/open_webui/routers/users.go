package routers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type UsersRuntimeConfig struct {
	WebUISecretKey string
	StaticDir      string
}

type UsersRouter struct {
	Config UsersRuntimeConfig
	Users  *models.UsersTable
	Auths  *models.AuthsTable
}

type userInfoResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Role            string `json:"role"`
	ProfileImageURL string `json:"profile_image_url"`
	IsActive        bool   `json:"is_active"`
	Groups          []any  `json:"groups"`
}

type userActiveResponse struct {
	ID                    string         `json:"id"`
	Email                 string         `json:"email"`
	Username              string         `json:"username,omitempty"`
	Role                  string         `json:"role"`
	Name                  string         `json:"name"`
	ProfileImageURL       string         `json:"profile_image_url"`
	ProfileBannerImageURL string         `json:"profile_banner_image_url,omitempty"`
	Bio                   string         `json:"bio,omitempty"`
	Gender                string         `json:"gender,omitempty"`
	DateOfBirth           string         `json:"date_of_birth,omitempty"`
	Timezone              string         `json:"timezone,omitempty"`
	PresenceState         string         `json:"presence_state,omitempty"`
	StatusEmoji           string         `json:"status_emoji,omitempty"`
	StatusMessage         string         `json:"status_message,omitempty"`
	StatusExpiresAt       *int64         `json:"status_expires_at,omitempty"`
	Info                  map[string]any `json:"info,omitempty"`
	Settings              map[string]any `json:"settings,omitempty"`
	OAuth                 map[string]any `json:"oauth,omitempty"`
	SCIM                  map[string]any `json:"scim,omitempty"`
	LastActiveAt          int64          `json:"last_active_at"`
	UpdatedAt             int64          `json:"updated_at"`
	CreatedAt             int64          `json:"created_at"`
	Groups                []any          `json:"groups"`
	IsActive              bool           `json:"is_active"`
}

type updateUserForm struct {
	Role            string  `json:"role"`
	Name            string  `json:"name"`
	Email           string  `json:"email"`
	ProfileImageURL string  `json:"profile_image_url"`
	Password        *string `json:"password"`
}

type userStatusForm struct {
	StatusEmoji     *string `json:"status_emoji"`
	StatusMessage   *string `json:"status_message"`
	StatusExpiresAt *int64  `json:"status_expires_at"`
}

func (h *UsersRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/users/user/settings", h.GetSessionUserSettings)
	mux.HandleFunc("POST /api/v1/users/user/settings/update", h.UpdateSessionUserSettings)
	mux.HandleFunc("GET /api/v1/users/user/status", h.GetSessionUserStatus)
	mux.HandleFunc("POST /api/v1/users/user/status/update", h.UpdateSessionUserStatus)
	mux.HandleFunc("GET /api/v1/users/user/info", h.GetSessionUserInfo)
	mux.HandleFunc("POST /api/v1/users/user/info/update", h.UpdateSessionUserInfo)
	mux.HandleFunc("GET /api/v1/users/{user_id}", h.GetUserByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/active", h.GetUserActiveStatusByID)
	mux.HandleFunc("POST /api/v1/users/{user_id}/update", h.UpdateUserByID)
	mux.HandleFunc("DELETE /api/v1/users/{user_id}", h.DeleteUserByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/info", h.GetUserInfoByID)
	mux.HandleFunc("GET /api/v1/users/{user_id}/profile/image", h.GetUserProfileImageByID)
}

func (h *UsersRouter) GetSessionUserSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user.Settings)
}

func (h *UsersRouter) UpdateSessionUserSettings(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Users.UpdateUserSettingsByID(r.Context(), user.ID, payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated.Settings)
}

func (h *UsersRouter) GetSessionUserStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(user))
}

func (h *UsersRouter) UpdateSessionUserStatus(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var form userStatusForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	updated, err := h.Users.UpdateUserByID(r.Context(), user.ID, models.UserUpdateParams{
		StatusEmoji:     form.StatusEmoji,
		StatusMessage:   form.StatusMessage,
		StatusExpiresAt: form.StatusExpiresAt,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(updated))
}

func (h *UsersRouter) GetSessionUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, user.Info)
}

func (h *UsersRouter) UpdateSessionUserInfo(w http.ResponseWriter, r *http.Request) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}

	var payload map[string]any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}
	updated, err := h.Users.UpdateUserInfoByID(r.Context(), user.ID, payload)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updated == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, updated.Info)
}

func (h *UsersRouter) GetUserInfoByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, userInfoResponse{
		ID:              user.ID,
		Name:            user.Name,
		Email:           user.Email,
		Role:            user.Role,
		ProfileImageURL: user.ProfileImageURL,
		IsActive:        false,
		Groups:          []any{},
	})
}

func (h *UsersRouter) GetUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	writeJSON(w, http.StatusOK, serializeUser(user))
}

func (h *UsersRouter) GetUserActiveStatusByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"active": false})
}

func (h *UsersRouter) GetUserProfileImageByID(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireVerifiedUser(w, r); !ok {
		return
	}

	userID := r.PathValue("user_id")
	user, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if user == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}

	switch {
	case strings.HasPrefix(user.ProfileImageURL, "http://"), strings.HasPrefix(user.ProfileImageURL, "https://"):
		http.Redirect(w, r, user.ProfileImageURL, http.StatusFound)
		return
	case strings.HasPrefix(user.ProfileImageURL, "data:image"):
		header, base64Data, found := strings.Cut(user.ProfileImageURL, ",")
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image"})
			return
		}
		imageData, err := base64.StdEncoding.DecodeString(base64Data)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image"})
			return
		}
		mediaType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
		w.Header().Set("Content-Type", mediaType)
		_, _ = io.Copy(w, bytes.NewReader(imageData))
		return
	default:
		http.ServeFile(w, r, filepath.Join(h.Config.StaticDir, "user.png"))
	}
}

func (h *UsersRouter) UpdateUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	var form updateUserForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	firstUser, err := h.Users.GetFirstUser(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "could not verify primary admin status"})
		return
	}
	if firstUser != nil && userID == firstUser.ID {
		if sessionUser.ID != userID {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
			return
		}
		if form.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
			return
		}
	}

	currentUser, err := h.Users.GetUserByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if currentUser == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	if !utils.ValidateProfileImageURL(form.ProfileImageURL) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid profile image url"})
		return
	}

	form.Email = strings.ToLower(strings.TrimSpace(form.Email))
	emailUser, err := h.Users.GetUserByEmail(r.Context(), form.Email)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if emailUser != nil && emailUser.ID != currentUser.ID {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "email already taken"})
		return
	}

	if form.Password != nil && strings.TrimSpace(*form.Password) != "" {
		hashed, err := utils.GetPasswordHash(*form.Password)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": err.Error()})
			return
		}
		if _, err := h.Auths.UpdateUserPasswordByID(r.Context(), userID, hashed); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	if _, err := h.Auths.UpdateEmailByID(r.Context(), userID, form.Email); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}

	updatedUser, err := h.Users.UpdateUserByID(r.Context(), userID, models.UserUpdateParams{
		Role:            &form.Role,
		Name:            &form.Name,
		Email:           &form.Email,
		ProfileImageURL: &form.ProfileImageURL,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if updatedUser == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "user not found"})
		return
	}
	writeJSON(w, http.StatusOK, serializeUser(updatedUser))
}

func (h *UsersRouter) DeleteUserByID(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return
	}
	if sessionUser.Role != "admin" {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	userID := r.PathValue("user_id")
	firstUser, err := h.Users.GetFirstUser(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "could not verify primary admin status"})
		return
	}
	if firstUser != nil && userID == firstUser.ID {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}
	if sessionUser.ID == userID {
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "action prohibited"})
		return
	}

	deleted, err := h.Auths.DeleteAuthByID(r.Context(), userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	if !deleted {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": "delete user error"})
		return
	}
	writeJSON(w, http.StatusOK, true)
}

func (h *UsersRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	token := utils.ExtractTokenFromRequest(r)
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}

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
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "invalid token"})
		return nil, false
	}
	return user, true
}

func serializeUser(user *models.User) userActiveResponse {
	return userActiveResponse{
		ID:                    user.ID,
		Email:                 user.Email,
		Username:              user.Username,
		Role:                  user.Role,
		Name:                  user.Name,
		ProfileImageURL:       user.ProfileImageURL,
		ProfileBannerImageURL: user.ProfileBannerImageURL,
		Bio:                   user.Bio,
		Gender:                user.Gender,
		DateOfBirth:           user.DateOfBirth,
		Timezone:              user.Timezone,
		PresenceState:         user.PresenceState,
		StatusEmoji:           user.StatusEmoji,
		StatusMessage:         user.StatusMessage,
		StatusExpiresAt:       user.StatusExpiresAt,
		Info:                  user.Info,
		Settings:              user.Settings,
		OAuth:                 user.OAuth,
		SCIM:                  user.SCIM,
		LastActiveAt:          user.LastActiveAt,
		UpdatedAt:             user.UpdatedAt,
		CreatedAt:             user.CreatedAt,
		Groups:                []any{},
		IsActive:              false,
	}
}
