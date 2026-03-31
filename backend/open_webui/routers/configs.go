package routers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/utils"
)

type ConfigsStore interface {
	Load(ctx context.Context) (map[string]any, error)
	Save(ctx context.Context, data map[string]any) error
}

type ConfigsState struct {
	EnableDirectConnections   bool
	EnableBaseModelsCache     bool
	ToolServerConnections     []map[string]any
	TerminalServerConnections []map[string]any
}

type ConfigsRuntimeConfig struct {
	WebUISecretKey string
	EnableAPIKeys  bool
	State          *ConfigsState
}

type ConfigsRouter struct {
	Config ConfigsRuntimeConfig
	Users  *models.UsersTable
	Store  ConfigsStore
}

type importConfigForm struct {
	Config map[string]any `json:"config"`
}

type connectionsConfigForm struct {
	EnableDirectConnections bool `json:"ENABLE_DIRECT_CONNECTIONS"`
	EnableBaseModelsCache   bool `json:"ENABLE_BASE_MODELS_CACHE"`
}

type terminalServersConfigForm struct {
	TerminalServerConnections []map[string]any `json:"TERMINAL_SERVER_CONNECTIONS"`
}

type toolServersConfigForm struct {
	ToolServerConnections []map[string]any `json:"TOOL_SERVER_CONNECTIONS"`
}

func (h *ConfigsRouter) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/configs/import", h.ImportConfig)
	mux.HandleFunc("GET /api/v1/configs/export", h.ExportConfig)
	mux.HandleFunc("GET /api/v1/configs/connections", h.GetConnectionsConfig)
	mux.HandleFunc("POST /api/v1/configs/connections", h.SetConnectionsConfig)
	mux.HandleFunc("GET /api/v1/configs/tool_servers", h.GetToolServersConfig)
	mux.HandleFunc("POST /api/v1/configs/tool_servers", h.SetToolServersConfig)
	mux.HandleFunc("GET /api/v1/configs/terminal_servers", h.GetTerminalServersConfig)
	mux.HandleFunc("POST /api/v1/configs/terminal_servers", h.SetTerminalServersConfig)
}

func (h *ConfigsRouter) ImportConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form importConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil || form.Config == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData := cloneConfigPayload(form.Config)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		loaded, err := h.loadConfig(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
		h.applyConfig(loaded)
		writeJSON(w, http.StatusOK, loaded)
		return
	}

	h.applyConfig(configData)
	writeJSON(w, http.StatusOK, configData)
}

func (h *ConfigsRouter) ExportConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, configData)
}

func (h *ConfigsRouter) GetConnectionsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.connectionsResponse())
}

func (h *ConfigsRouter) SetConnectionsConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form connectionsConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "direct.enable", form.EnableDirectConnections)
	setConfigPathValue(configData, "models.base_models_cache", form.EnableBaseModelsCache)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	state := h.state()
	state.EnableDirectConnections = form.EnableDirectConnections
	state.EnableBaseModelsCache = form.EnableBaseModelsCache
	writeJSON(w, http.StatusOK, h.connectionsResponse())
}

func (h *ConfigsRouter) GetTerminalServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.terminalServersResponse())
}

func (h *ConfigsRouter) GetToolServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, h.toolServersResponse())
}

func (h *ConfigsRouter) SetToolServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form toolServersConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "tool_server.connections", form.ToolServerConnections)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	h.state().ToolServerConnections = cloneConfigList(form.ToolServerConnections)
	writeJSON(w, http.StatusOK, h.toolServersResponse())
}

func (h *ConfigsRouter) SetTerminalServersConfig(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminUser(w, r); !ok {
		return
	}
	var form terminalServersConfigForm
	if err := json.NewDecoder(r.Body).Decode(&form); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid request body"})
		return
	}

	configData, err := h.loadConfig(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
		return
	}
	setConfigPathValue(configData, "terminal_server.connections", form.TerminalServerConnections)
	if h.Store != nil {
		if err := h.Store.Save(r.Context(), configData); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"detail": err.Error()})
			return
		}
	}

	h.state().TerminalServerConnections = cloneConfigList(form.TerminalServerConnections)
	writeJSON(w, http.StatusOK, h.terminalServersResponse())
}

func (h *ConfigsRouter) loadConfig(ctx context.Context) (map[string]any, error) {
	if h.Store == nil {
		return defaultConfigPayload(), nil
	}
	configData, err := h.Store.Load(ctx)
	if err != nil {
		return nil, err
	}
	if configData == nil {
		return defaultConfigPayload(), nil
	}
	return cloneConfigPayload(configData), nil
}

func (h *ConfigsRouter) applyConfig(configData map[string]any) {
	state := h.state()
	if value, ok := getConfigPathValue(configData, "direct.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableDirectConnections = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "models.base_models_cache"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			state.EnableBaseModelsCache = enabled
		}
	}
	if value, ok := getConfigPathValue(configData, "tool_server.connections"); ok {
		if connections, typeOK := normalizeConfigList(value); typeOK {
			state.ToolServerConnections = connections
		}
	}
	if value, ok := getConfigPathValue(configData, "terminal_server.connections"); ok {
		if connections, typeOK := normalizeConfigList(value); typeOK {
			state.TerminalServerConnections = connections
		}
	}
}

func (h *ConfigsRouter) connectionsResponse() connectionsConfigForm {
	state := h.state()
	return connectionsConfigForm{
		EnableDirectConnections: state.EnableDirectConnections,
		EnableBaseModelsCache:   state.EnableBaseModelsCache,
	}
}

func (h *ConfigsRouter) terminalServersResponse() terminalServersConfigForm {
	return terminalServersConfigForm{
		TerminalServerConnections: cloneConfigList(h.state().TerminalServerConnections),
	}
}

func (h *ConfigsRouter) toolServersResponse() toolServersConfigForm {
	return toolServersConfigForm{
		ToolServerConnections: cloneConfigList(h.state().ToolServerConnections),
	}
}

func (h *ConfigsRouter) state() *ConfigsState {
	if h.Config.State == nil {
		h.Config.State = &ConfigsState{}
	}
	return h.Config.State
}

func (h *ConfigsRouter) requireVerifiedUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
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

func (h *ConfigsRouter) requireAdminUser(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, ok := h.requireVerifiedUser(w, r)
	if !ok {
		return nil, false
	}
	if user.Role != "admin" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "access prohibited"})
		return nil, false
	}
	return user, true
}

func defaultConfigPayload() map[string]any {
	return map[string]any{
		"version": 0,
		"ui":      map[string]any{},
	}
}

func cloneConfigPayload(source map[string]any) map[string]any {
	body, err := json.Marshal(source)
	if err != nil {
		return map[string]any{}
	}
	var target map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return map[string]any{}
	}
	return target
}

func cloneConfigList(source []map[string]any) []map[string]any {
	if source == nil {
		return []map[string]any{}
	}
	body, err := json.Marshal(source)
	if err != nil {
		return []map[string]any{}
	}
	var target []map[string]any
	if err := json.Unmarshal(body, &target); err != nil {
		return []map[string]any{}
	}
	return target
}

func getConfigPathValue(configData map[string]any, path string) (any, bool) {
	current := any(configData)
	for _, part := range strings.Split(path, ".") {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		value, exists := node[part]
		if !exists {
			return nil, false
		}
		current = value
	}
	return current, true
}

func setConfigPathValue(configData map[string]any, path string, value any) {
	current := configData
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	current[parts[len(parts)-1]] = value
}

func normalizeConfigList(value any) ([]map[string]any, bool) {
	if value == nil {
		return []map[string]any{}, true
	}
	body, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, false
	}
	return items, true
}
