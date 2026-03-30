package openwebui

import (
	"errors"
	"net/http"
	"time"

	"github.com/xxnuo/open-coreui/backend/internal/platform/proxy"
)

func NewHandler(cfg RuntimeConfig) (http.Handler, error) {
	if cfg.PythonBaseURL == "" {
		return nil, errors.New("python base url is required")
	}

	upstream, err := proxy.New(cfg.PythonBaseURL)
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
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
