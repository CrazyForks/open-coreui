package openwebui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/xxnuo/open-coreui/backend/internal/platform/proxy"
	dbinternal "github.com/xxnuo/open-coreui/backend/open_webui/internal"
	"github.com/xxnuo/open-coreui/backend/open_webui/migrations"
	"github.com/xxnuo/open-coreui/backend/open_webui/models"
	"github.com/xxnuo/open-coreui/backend/open_webui/routers"
	"github.com/xxnuo/open-coreui/backend/open_webui/storage"
)

func NewHandler(cfg RuntimeConfig) (http.Handler, error) {
	if cfg.PythonBaseURL == "" {
		return nil, errors.New("python base url is required")
	}

	ctx := context.Background()
	db, err := dbinternal.Open(ctx, dbinternal.Options{
		DatabaseURL:     cfg.DatabaseURL,
		DatabaseSchema:  cfg.DatabaseSchema,
		EnableSQLiteWAL: cfg.DatabaseEnableSQLiteWAL,
		PoolSize:        cfg.DatabasePoolSize,
		PoolRecycle:     cfg.DatabasePoolRecycle,
		OpenTimeout:     cfg.DatabasePoolTimeout,
	})
	if err != nil {
		return nil, err
	}
	if cfg.EnableDBMigrations {
		if err := migrations.Run(ctx, db); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	configStore, err := NewConfigStoreWithHandle(ctx, cfg, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	configsState := &routers.ConfigsState{
		EnableDirectConnections:   cfg.EnableDirectConnections,
		EnableBaseModelsCache:     cfg.EnableBaseModelsCache,
		ToolServerConnections:     cloneRuntimeConfigList(cfg.ToolServerConnections),
		TerminalServerConnections: cloneRuntimeConfigList(cfg.TerminalServerConnections),
	}
	configData, err := configStore.Load(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if value, ok := GetConfigValue(configData, "direct.enable"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableDirectConnections = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "models.base_models_cache"); ok {
		if enabled, typeOK := value.(bool); typeOK {
			configsState.EnableBaseModelsCache = enabled
		}
	}
	if value, ok := GetConfigValue(configData, "tool_server.connections"); ok {
		if connections, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.ToolServerConnections = connections
		}
	}
	if value, ok := GetConfigValue(configData, "terminal_server.connections"); ok {
		if connections, typeOK := decodeRuntimeConfigList(value); typeOK {
			configsState.TerminalServerConnections = connections
		}
	}

	upstream, err := proxy.New(cfg.PythonBaseURL)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	usersTable := models.NewUsersTable(db)
	authRouter := &routers.AuthsRouter{
		Config: routers.AuthRuntimeConfig{
			WebUIAuth:                cfg.WebUIAuth,
			EnableInitialAdminSignup: cfg.EnableInitialAdminSignup,
			EnablePasswordAuth:       cfg.EnablePasswordAuth,
			EnableAPIKeys:            cfg.EnableAPIKeys,
			EnableSignup:             cfg.EnableSignup,
			DefaultUserRole:          cfg.DefaultUserRole,
			ShowAdminDetails:         cfg.ShowAdminDetails,
			AdminEmail:               cfg.AdminEmail,
			WebUISecretKey:           cfg.WebUISecretKey,
			JWTExpiresIn:             cfg.JWTExpiresIn,
			AuthCookieSameSite:       cfg.AuthCookieSameSite,
			AuthCookieSecure:         cfg.AuthCookieSecure,
			TrustedEmailHeader:       cfg.TrustedEmailHeader,
		},
		Users: usersTable,
		Auths: models.NewAuthsTable(db, usersTable),
	}
	authRouter.Register(mux)
	usersRouter := &routers.UsersRouter{
		Config: routers.UsersRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			StaticDir:      cfg.StaticDir,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:         usersTable,
		Auths:         authRouter.Auths,
		Groups:        models.NewGroupsTable(db),
		OAuthSessions: models.NewOAuthSessionsTable(db),
	}
	usersRouter.Register(mux)
	groupsTable := models.NewGroupsTable(db)
	accessGrantsTable := models.NewAccessGrantsTable(db)
	groupsRouter := &routers.GroupsRouter{
		Config: routers.GroupsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:  usersTable,
		Groups: groupsTable,
	}
	groupsRouter.Register(mux)
	notesRouter := &routers.NotesRouter{
		Config: routers.NotesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Notes:        models.NewNotesTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	notesRouter.Register(mux)
	memoriesRouter := &routers.MemoriesRouter{
		Config: routers.MemoriesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:    usersTable,
		Memories: models.NewMemoriesTable(db),
	}
	memoriesRouter.Register(mux)
	foldersRouter := &routers.FoldersRouter{
		Config: routers.FoldersRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:   usersTable,
		Folders: models.NewFoldersTable(db),
	}
	foldersRouter.Register(mux)
	promptsRouter := &routers.PromptsRouter{
		Config: routers.PromptsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Prompts:      models.NewPromptsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	promptsRouter.Register(mux)
	functionsRouter := &routers.FunctionsRouter{
		Config: routers.FunctionsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:     usersTable,
		Functions: models.NewFunctionsTable(db),
	}
	functionsRouter.Register(mux)
	skillsRouter := &routers.SkillsRouter{
		Config: routers.SkillsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Skills:       models.NewSkillsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	skillsRouter.Register(mux)
	modelsRouter := &routers.ModelsRouter{
		Config: routers.ModelsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			StaticDir:      cfg.StaticDir,
		},
		Users:        usersTable,
		Models:       models.NewModelsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	modelsRouter.Register(mux)
	filesRouter := &routers.FilesRouter{
		Config: routers.FilesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Files:        models.NewFilesTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
		Storage:      storage.NewLocalProvider(cfg.UploadDir),
	}
	filesRouter.Register(mux)
	evaluationsRouter := &routers.EvaluationsRouter{
		Config: &routers.EvaluationsRuntimeConfig{
			WebUISecretKey:              cfg.WebUISecretKey,
			EnableAPIKeys:               cfg.EnableAPIKeys,
			EnableEvaluationArenaModels: cfg.EnableEvaluationArenaModels,
			EvaluationArenaModels:       cfg.EvaluationArenaModels,
		},
		Users:     usersTable,
		Feedbacks: models.NewFeedbacksTable(db),
	}
	evaluationsRouter.Register(mux)
	toolsRouter := &routers.ToolsRouter{
		Config: routers.ToolsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:        usersTable,
		Tools:        models.NewToolsTable(db),
		Groups:       groupsTable,
		AccessGrants: accessGrantsTable,
	}
	toolsRouter.Register(mux)
	configsRouter := &routers.ConfigsRouter{
		Config: routers.ConfigsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			State:          configsState,
		},
		Users: usersTable,
		Store: configStore,
	}
	configsRouter.Register(mux)
	terminalsRouter := &routers.TerminalsRouter{
		Config: routers.TerminalsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			State:          configsState,
		},
		Users:  usersTable,
		Groups: groupsTable,
	}
	terminalsRouter.Register(mux)
	utilsRouter := &routers.UtilsRouter{
		Config: routers.UtilsRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
			DatabaseURL:    cfg.DatabaseURL,
		},
		Users: usersTable,
	}
	utilsRouter.Register(mux)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("/", upstream)

	return mux, nil
}

func Run() error {
	cfg := ConfigFromEnv()
	handler, err := NewHandler(cfg)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}

func cloneRuntimeConfigList(source []map[string]any) []map[string]any {
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

func decodeRuntimeConfigList(value any) ([]map[string]any, bool) {
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
