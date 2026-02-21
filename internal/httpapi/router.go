package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

const (
	configBootstrapMessage = "No admin user configured. Use POST /config to bootstrap first admin."
	configLoginMessage     = "Instance configured. Please login using POST /auth/login."
)

type Logger interface {
	Printf(format string, args ...any)
}

type MetadataPinger interface {
	Ping(ctx context.Context) error
}

type Dependencies struct {
	Config                  config.Config
	AuthService             coreauth.Service
	MetadataPinger          MetadataPinger
	Logger                  Logger
	EncryptionKeyConfigured bool
	JWTSigningKeyConfigured bool
}

func RegisterRoutes(mux *http.ServeMux, deps Dependencies) error {
	if mux == nil {
		return errors.New("mux is required")
	}
	if deps.AuthService == nil {
		return errors.New("auth service is required")
	}
	if deps.MetadataPinger == nil {
		return errors.New("metadata pinger is required")
	}
	if deps.Logger == nil {
		return errors.New("logger is required")
	}

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := deps.MetadataPinger.Ping(ctx); err != nil {
			http.Error(w, "metadata store unavailable", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	mux.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetConfig(w, r, deps)
		case http.MethodPost:
			handleBootstrapConfig(w, r, deps)
		default:
			writeMethodNotAllowed(w)
		}
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !requireMethod(w, r, http.MethodPost) {
			return
		}
		handleAuthLogin(w, r, deps)
	})

	return nil
}
