package openwebui

import (
	"context"
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
		Users: usersTable,
		Notes: models.NewNotesTable(db),
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
		Users:  usersTable,
		Models: models.NewModelsTable(db),
	}
	modelsRouter.Register(mux)
	filesRouter := &routers.FilesRouter{
		Config: routers.FilesRuntimeConfig{
			WebUISecretKey: cfg.WebUISecretKey,
			EnableAPIKeys:  cfg.EnableAPIKeys,
		},
		Users:   usersTable,
		Files:   models.NewFilesTable(db),
		Storage: storage.NewLocalProvider(cfg.UploadDir),
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
		Users: usersTable,
		Tools: models.NewToolsTable(db),
	}
	toolsRouter.Register(mux)
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
