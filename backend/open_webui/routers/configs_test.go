package routers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
)

type testConfigsStore struct {
	data map[string]any
}

func (s *testConfigsStore) Load(context.Context) (map[string]any, error) {
	if s.data == nil {
		return nil, nil
	}
	return cloneConfigPayload(s.data), nil
}

func (s *testConfigsStore) Save(_ context.Context, data map[string]any) error {
	s.data = cloneConfigPayload(data)
	return nil
}

func TestConfigsRouterImportExportConnections(t *testing.T) {
	t.Parallel()

	db := openRouterTestDB(t)
	defer db.Close()

	users := models.NewUsersTable(db)
	auths := models.NewAuthsTable(db, users)
	store := &testConfigsStore{}
	state := &ConfigsState{}

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

	configsRouter := &ConfigsRouter{
		Config: ConfigsRuntimeConfig{
			WebUISecretKey: "secret",
			EnableAPIKeys:  true,
			State:          state,
		},
		Users: users,
		Store: store,
	}

	mux := http.NewServeMux()
	authRouter.Register(mux)
	configsRouter.Register(mux)

	signupBody, _ := json.Marshal(map[string]any{
		"name":              "Config User",
		"email":             "config@example.com",
		"password":          "password-123",
		"profile_image_url": "/user.png",
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

	exportReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes := httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status: %d", exportRes.Code)
	}

	importBody, _ := json.Marshal(map[string]any{
		"config": map[string]any{
			"version": 2,
			"ui": map[string]any{
				"theme": "dark",
			},
			"direct": map[string]any{
				"enable": true,
			},
			"models": map[string]any{
				"base_models_cache": true,
			},
		},
	})
	importReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/import", bytes.NewReader(importBody))
	importReq.Header.Set("Authorization", "Bearer "+token)
	importRes := httptest.NewRecorder()
	mux.ServeHTTP(importRes, importReq)
	if importRes.Code != http.StatusOK {
		t.Fatalf("unexpected import status: %d", importRes.Code)
	}
	if !state.EnableDirectConnections || !state.EnableBaseModelsCache {
		t.Fatal("expected config import to update runtime state")
	}

	connectionsReq := httptest.NewRequest(http.MethodGet, "/api/v1/configs/connections", nil)
	connectionsReq.Header.Set("Authorization", "Bearer "+token)
	connectionsRes := httptest.NewRecorder()
	mux.ServeHTTP(connectionsRes, connectionsReq)
	if connectionsRes.Code != http.StatusOK {
		t.Fatalf("unexpected connections status: %d", connectionsRes.Code)
	}

	updateBody, _ := json.Marshal(map[string]any{
		"ENABLE_DIRECT_CONNECTIONS": false,
		"ENABLE_BASE_MODELS_CACHE":  false,
	})
	updateReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/connections", bytes.NewReader(updateBody))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateRes := httptest.NewRecorder()
	mux.ServeHTTP(updateRes, updateReq)
	if updateRes.Code != http.StatusOK {
		t.Fatalf("unexpected update connections status: %d", updateRes.Code)
	}
	if state.EnableDirectConnections || state.EnableBaseModelsCache {
		t.Fatal("expected connections update to update runtime state")
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after update: %d", exportRes.Code)
	}

	var exportPayload map[string]any
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	direct, ok := getConfigPathValue(exportPayload, "direct.enable")
	if !ok || direct != false {
		t.Fatalf("unexpected direct.enable: %v", direct)
	}
	cache, ok := getConfigPathValue(exportPayload, "models.base_models_cache")
	if !ok || cache != false {
		t.Fatalf("unexpected models.base_models_cache: %v", cache)
	}

	terminalServersBody, _ := json.Marshal(map[string]any{
		"TERMINAL_SERVER_CONNECTIONS": []map[string]any{
			{
				"id":      "term-1",
				"url":     "http://127.0.0.1:19000",
				"name":    "Terminal 1",
				"enabled": true,
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
		},
	})
	terminalServersReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/terminal_servers", bytes.NewReader(terminalServersBody))
	terminalServersReq.Header.Set("Authorization", "Bearer "+token)
	terminalServersRes := httptest.NewRecorder()
	mux.ServeHTTP(terminalServersRes, terminalServersReq)
	if terminalServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal servers update status: %d", terminalServersRes.Code)
	}
	if len(state.TerminalServerConnections) != 1 {
		t.Fatalf("unexpected terminal server state: %d", len(state.TerminalServerConnections))
	}

	terminalServersReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/terminal_servers", nil)
	terminalServersReq.Header.Set("Authorization", "Bearer "+token)
	terminalServersRes = httptest.NewRecorder()
	mux.ServeHTTP(terminalServersRes, terminalServersReq)
	if terminalServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected terminal servers status: %d", terminalServersRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after terminal servers update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	terminalConnections, ok := getConfigPathValue(exportPayload, "terminal_server.connections")
	if !ok {
		t.Fatal("missing terminal_server.connections")
	}
	connections, ok := normalizeConfigList(terminalConnections)
	if !ok || len(connections) != 1 {
		t.Fatalf("unexpected terminal_server.connections: %v", terminalConnections)
	}

	toolServersBody, _ := json.Marshal(map[string]any{
		"TOOL_SERVER_CONNECTIONS": []map[string]any{
			{
				"url":       "http://127.0.0.1:20000",
				"path":      "/openapi.json",
				"type":      "openapi",
				"auth_type": "none",
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
		},
	})
	toolServersReq := httptest.NewRequest(http.MethodPost, "/api/v1/configs/tool_servers", bytes.NewReader(toolServersBody))
	toolServersReq.Header.Set("Authorization", "Bearer "+token)
	toolServersRes := httptest.NewRecorder()
	mux.ServeHTTP(toolServersRes, toolServersReq)
	if toolServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool servers update status: %d", toolServersRes.Code)
	}
	if len(state.ToolServerConnections) != 1 {
		t.Fatalf("unexpected tool server state: %d", len(state.ToolServerConnections))
	}

	toolServersReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/tool_servers", nil)
	toolServersReq.Header.Set("Authorization", "Bearer "+token)
	toolServersRes = httptest.NewRecorder()
	mux.ServeHTTP(toolServersRes, toolServersReq)
	if toolServersRes.Code != http.StatusOK {
		t.Fatalf("unexpected tool servers status: %d", toolServersRes.Code)
	}

	exportReq = httptest.NewRequest(http.MethodGet, "/api/v1/configs/export", nil)
	exportReq.Header.Set("Authorization", "Bearer "+token)
	exportRes = httptest.NewRecorder()
	mux.ServeHTTP(exportRes, exportReq)
	if exportRes.Code != http.StatusOK {
		t.Fatalf("unexpected export status after tool servers update: %d", exportRes.Code)
	}
	if err := json.Unmarshal(exportRes.Body.Bytes(), &exportPayload); err != nil {
		t.Fatal(err)
	}
	toolConnections, ok := getConfigPathValue(exportPayload, "tool_server.connections")
	if !ok {
		t.Fatal("missing tool_server.connections")
	}
	connections, ok = normalizeConfigList(toolConnections)
	if !ok || len(connections) != 1 {
		t.Fatalf("unexpected tool_server.connections: %v", toolConnections)
	}
}
