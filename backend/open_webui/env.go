package openwebui

import "os"

type Config struct {
	ListenAddr    string
	PythonBaseURL string
}

func ConfigFromEnv() Config {
	listenAddr := os.Getenv("OPEN_COREUI_GO_ADDR")
	if listenAddr == "" {
		listenAddr = ":8081"
	}

	pythonBaseURL := os.Getenv("OPEN_COREUI_PYTHON_BASE_URL")
	if pythonBaseURL == "" {
		pythonBaseURL = "http://127.0.0.1:8080"
	}

	return Config{
		ListenAddr:    listenAddr,
		PythonBaseURL: pythonBaseURL,
	}
}
