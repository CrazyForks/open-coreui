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

func TestTerminalsRouterListAndProxy(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	groups := models.NewGroupsTable(db)

	authRouter := &AuthsRouter{
		Config: AuthRuntimeConfig{
			WebUIAuth:                true,
			EnableInitialAdminSignup: true,
			EnablePasswordAuth:       true,
			EnableAPIKeys:            true,
			EnableSignup:             true,
			DefaultUserRole:          "pending",
			ShowAdminDetails:         true,
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

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Upstream", "ok")
		if r.Header.Get("Authorization") != "Bearer term-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("bad auth"))
			return
		}
		payload, _ := json.Marshal(map[string]any{
			"path":    r.URL.Path,
			"query":   r.URL.RawQuery,
			"user_id": r.Header.Get("X-User-Id"),
		})
		_, _ = w.Write(payload)
	}))
	defer upstream.Close()

	state := &ConfigsState{
		TerminalServerConnections: []map[string]any{
			{
				"id":      "term-public",
				"url":     upstream.URL,
				"name":    "Public Terminal",
				"enabled": true,
				"key":     "term-secret",
				"config": map[string]any{
					"access_grants": []map[string]any{
						{
							"principal_type": "user",
							"principal_id":   "*",
							"permission":     "read",
						},
					},
				},
			},
			{
				"id":      "term-private",
				"url":     upstream.URL,
				"name":    "Private Terminal",
				"enabled": true,
			},
		},
	}

	terminalsRouter := &TerminalsRouter{
		Config: TerminalsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			State:          state,
			HTTPClient:     upstream.Client(),
		},
		Users:  users,
		Groups: groups,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	terminalsRouter.Register(mux)

	adminSignupBody, _ := json.Marshal(map[string]any{
		"name":              "Admin User",
		"email":             "admin-terminals@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	adminSignupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(adminSignupBody))
	adminSignupRes := httptest.NewRecorder()
	mux.ServeHTTP(adminSignupRes, adminSignupReq)
	if adminSignupRes.Code != http.StatusOK {
		t.Fatalf("unexpected admin signup status: %d", adminSignupRes.Code)
	}

	userSignupBody, _ := json.Marshal(map[string]any{
		"name":              "Terminal User",
		"email":             "user-terminals@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
	})
	userSignupReq := httptest.NewRequest(http.MethodPost, "/api/v1/auths/signup", bytes.NewReader(userSignupBody))
	userSignupRes := httptest.NewRecorder()
	mux.ServeHTTP(userSignupRes, userSignupReq)
	if userSignupRes.Code != http.StatusOK {
		t.Fatalf("unexpected user signup status: %d", userSignupRes.Code)
	}
	var userSignupPayload map[string]any
	if err := json.Unmarshal(userSignupRes.Body.Bytes(), &userSignupPayload); err != nil {
		t.Fatal(err)
	}
	userToken, _ := userSignupPayload["token"].(string)
	userID, _ := userSignupPayload["id"].(string)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/terminals/", nil)
	listReq.Header.Set("Authorization", "Bearer "+userToken)
	listRes := httptest.NewRecorder()
	mux.ServeHTTP(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal list status: %d", listRes.Code)
	}
	var listPayload []map[string]any
	if err := json.Unmarshal(listRes.Body.Bytes(), &listPayload); err != nil {
		t.Fatal(err)
	}
	if len(listPayload) != 1 {
		t.Fatalf("unexpected terminal list length: %d", len(listPayload))
	}
	if listPayload[0]["id"] != "term-public" {
		t.Fatalf("unexpected terminal list item: %v", listPayload[0])
	}

	proxyReq := httptest.NewRequest(http.MethodGet, "/api/v1/terminals/term-public/api/echo?x=1", nil)
	proxyReq.Header.Set("Authorization", "Bearer "+userToken)
	proxyRes := httptest.NewRecorder()
	mux.ServeHTTP(proxyRes, proxyReq)
	if proxyRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal proxy status: %d", proxyRes.Code)
	}
	var proxyPayload map[string]any
	if err := json.Unmarshal(proxyRes.Body.Bytes(), &proxyPayload); err != nil {
		t.Fatal(err)
	}
	if proxyPayload["path"] != "/api/echo" {
		t.Fatalf("unexpected proxied path: %v", proxyPayload["path"])
	}
	if proxyPayload["query"] != "x=1" {
		t.Fatalf("unexpected proxied query: %v", proxyPayload["query"])
	}
	if proxyPayload["user_id"] != userID {
		t.Fatalf("unexpected proxied user id: %v", proxyPayload["user_id"])
	}
}
