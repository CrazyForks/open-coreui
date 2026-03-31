package routers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

func TestUsersRouterInfoAndProfileImage(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	authRouter := &AuthsRouter{
		Config: AuthRuntimeConfig{
			WebUIAuth:                true,
			EnableInitialAdminSignup: true,
			EnablePasswordAuth:       true,
			WebUISecretKey:           "secret",
			JWTExpiresIn:             "1h",
			AuthCookieSameSite:       "Lax",
		},
		Users: users,
		Auths: auths,
		Now: func() time.Time {
			return time.Now().UTC()
		},
	}

	usersRouter := &UsersRouter{
		Config: UsersRuntimeConfig{
			WebUISecretKey: "secret",
			StaticDir:      "open-webui/backend/open_webui/static",
		},
		Users: users,
		Auths: auths,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	usersRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Tester",
		"email":             "tester@example.com",
		"password":          "password-123",
		"profile_image_url": "data:image/png;base64,Zm9v",
	})
	signupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(signupBody))
	signupRes := httptest.NewRecorder()
	mux.ServeHTTP(signupRes, signupReq)
	if signupRes.Code != http.StatusOK {
		t.Fatalf("unexpected signup status: %d", signupRes.Code)
	}

	var signupPayload map[string]any
	if err := json.Unmarshal(signupRes.Body.Bytes(), &signupPayload); err != nil {
		t.Fatal(err)
	}
	token, _ := signupPayload["token"].(string)
	userID, _ := signupPayload["id"].(string)

	infoReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID+"/info", nil)
	infoReq.Header.Set("Authorization", "Bearer "+token)
	infoRes := httptest.NewRecorder()
	mux.ServeHTTP(infoRes, infoReq)
	if infoRes.Code != http.StatusOK {
		t.Fatalf("unexpected info status: %d", infoRes.Code)
	}

	imageReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+userID+"/profile/image", nil)
	imageReq.Header.Set("Authorization", "Bearer "+token)
	imageRes := httptest.NewRecorder()
	mux.ServeHTTP(imageRes, imageReq)
	if imageRes.Code != http.StatusOK {
		t.Fatalf("unexpected image status: %d", imageRes.Code)
	}

	settingsUpdateBody, _ := json.Marshal(map[string]any{
		"ui": map[string]any{"theme": "light"},
	})
	settingsUpdateReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/user/settings/update", bytes.NewReader(settingsUpdateBody))
	settingsUpdateReq.Header.Set("Authorization", "Bearer "+token)
	settingsUpdateRes := httptest.NewRecorder()
	mux.ServeHTTP(settingsUpdateRes, settingsUpdateReq)
	if settingsUpdateRes.Code != http.StatusOK {
		t.Fatalf("unexpected settings update status: %d", settingsUpdateRes.Code)
	}

	settingsReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/settings", nil)
	settingsReq.Header.Set("Authorization", "Bearer "+token)
	settingsRes := httptest.NewRecorder()
	mux.ServeHTTP(settingsRes, settingsReq)
	if settingsRes.Code != http.StatusOK {
		t.Fatalf("unexpected settings status: %d", settingsRes.Code)
	}

	statusUpdateBody, _ := json.Marshal(map[string]any{
		"status_emoji":      ":)",
		"status_message":    "working",
		"status_expires_at": 123456789,
	})
	statusUpdateReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/user/status/update", bytes.NewReader(statusUpdateBody))
	statusUpdateReq.Header.Set("Authorization", "Bearer "+token)
	statusUpdateRes := httptest.NewRecorder()
	mux.ServeHTTP(statusUpdateRes, statusUpdateReq)
	if statusUpdateRes.Code != http.StatusOK {
		t.Fatalf("unexpected status update status: %d", statusUpdateRes.Code)
	}

	statusReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/status", nil)
	statusReq.Header.Set("Authorization", "Bearer "+token)
	statusRes := httptest.NewRecorder()
	mux.ServeHTTP(statusRes, statusReq)
	if statusRes.Code != http.StatusOK {
		t.Fatalf("unexpected status get status: %d", statusRes.Code)
	}

	infoUpdateBody, _ := json.Marshal(map[string]any{
		"nickname": "tester",
	})
	infoUpdateReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/user/info/update", bytes.NewReader(infoUpdateBody))
	infoUpdateReq.Header.Set("Authorization", "Bearer "+token)
	infoUpdateRes := httptest.NewRecorder()
	mux.ServeHTTP(infoUpdateRes, infoUpdateReq)
	if infoUpdateRes.Code != http.StatusOK {
		t.Fatalf("unexpected info update status: %d", infoUpdateRes.Code)
	}

	infoReqSession := httptest.NewRequest(http.MethodGet, "/api/v1/users/user/info", nil)
	infoReqSession.Header.Set("Authorization", "Bearer "+token)
	infoResSession := httptest.NewRecorder()
	mux.ServeHTTP(infoResSession, infoReqSession)
	if infoResSession.Code != http.StatusOK {
		t.Fatalf("unexpected info status: %d", infoResSession.Code)
	}

	secondSignupBody, _ := json.Marshal(map[string]any{
		"name":              "Tester Two",
		"email":             "tester2@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	secondSignupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(secondSignupBody))
	secondSignupRes := httptest.NewRecorder()
	mux.ServeHTTP(secondSignupRes, secondSignupReq)
	if secondSignupRes.Code != http.StatusOK {
		t.Fatalf("unexpected second signup status: %d", secondSignupRes.Code)
	}

	var secondSignupPayload map[string]any
	if err := json.Unmarshal(secondSignupRes.Body.Bytes(), &secondSignupPayload); err != nil {
		t.Fatal(err)
	}
	secondUserID, _ := secondSignupPayload["id"].(string)

	updateBody, _ := json.Marshal(map[string]any{
		"role":              "user",
		"name":              "Tester Two Updated",
		"email":             "tester2-updated@example.com",
		"profile_image_url": "/user.png",
		"password":          "password-789",
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/users/"+secondUserID+"/update", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected admin update status: %d", updateRes.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+secondUserID, nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRes := httptest.NewRecorder()
	mux.ServeHTTP(getRes, getReq)
	if getRes.Code != http.StatusOK {
		t.Fatalf("unexpected get user status: %d", getRes.Code)
	}

	activeReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/"+secondUserID+"/active", nil)
	activeReq.Header.Set("Authorization", "Bearer "+token)
	activeRes := httptest.NewRecorder()
	mux.ServeHTTP(activeRes, activeReq)
	if activeRes.Code != http.StatusOK {
		t.Fatalf("unexpected active status code: %d", activeRes.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/users/"+secondUserID, nil)
	deleteReq.Header.Set("Authorization", "Bearer "+token)
	deleteRes := httptest.NewRecorder()
	mux.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusOK {
		t.Fatalf("unexpected delete user status: %d", deleteRes.Code)
	}
}
