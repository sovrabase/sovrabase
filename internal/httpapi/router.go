package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

type bootstrapRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type apiError struct {
	Error string `json:"error"`
}

type configResponse struct {
	BootstrapRequired bool              `json:"bootstrap_required"`
	Message           string            `json:"message"`
	Auth              configAuthSection `json:"auth"`
	Config            sanitizedConfig   `json:"config"`
}

type configAuthSection struct {
	BootstrapEndpoint string `json:"bootstrap_endpoint"`
	LoginEndpoint     string `json:"login_endpoint"`
	Mode              string `json:"mode"`
}

type sanitizedConfig struct {
	Server       sanitizedServer       `json:"server"`
	Metadata     sanitizedMetadata     `json:"metadata_store"`
	Core         sanitizedCore         `json:"core"`
	Auth         sanitizedAuth         `json:"auth"`
	Provisioning sanitizedProvisioning `json:"provisioning"`
}

type sanitizedServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type sanitizedMetadata struct {
	Driver             string `json:"driver"`
	SQLiteConfigured   bool   `json:"sqlite_configured"`
	PostgresConfigured bool   `json:"postgres_configured"`
}

type sanitizedCore struct {
	CacheTTL                string `json:"cache_ttl"`
	SweepInterval           string `json:"sweep_interval"`
	EncryptionKeyConfigured bool   `json:"encryption_key_configured"`
}

type sanitizedAuth struct {
	JWTSigningKeyConfigured bool `json:"jwt_signing_key_configured"`
}

type sanitizedProvisioning struct {
	DefaultProvider string          `json:"default_provider"`
	Docker          sanitizedDocker `json:"docker"`
}

type sanitizedDocker struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	PostgresImage string `json:"postgres_image"`
	MongoImage    string `json:"mongo_image"`
	Endpoint      string `json:"endpoint"`
	HostAddress   string `json:"host_address"`
	NetworkName   string `json:"network_name"`
}

type bootstrapSuccessResponse struct {
	Message     string `json:"message"`
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	User        struct {
		ID    string            `json:"id"`
		Email string            `json:"email"`
		Role  coreauth.UserRole `json:"role"`
	} `json:"user"`
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

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
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
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleAuthLogin(w, r, deps)
	})

	return nil
}

func handleGetConfig(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	required, err := deps.AuthService.GetConfigState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "failed to resolve bootstrap state"})
		return
	}

	mode := "login"
	message := configLoginMessage
	if required {
		mode = "bootstrap"
		message = configBootstrapMessage
		deps.Logger.Printf("bootstrap still required: use POST /config to create first admin")
	}

	response := configResponse{
		BootstrapRequired: required,
		Message:           message,
		Auth: configAuthSection{
			BootstrapEndpoint: "/config",
			LoginEndpoint:     "/auth/login",
			Mode:              mode,
		},
		Config: sanitizeConfig(deps.Config, deps.EncryptionKeyConfigured, deps.JWTSigningKeyConfigured),
	}
	writeJSON(w, http.StatusOK, response)
}

func handleBootstrapConfig(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	var req bootstrapRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	result, err := deps.AuthService.BootstrapFirstAdmin(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid bootstrap payload"})
		case errors.Is(err, coreauth.ErrBootstrapAlreadyDone):
			writeJSON(w, http.StatusConflict, apiError{Error: "bootstrap already completed"})
		default:
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "bootstrap failed"})
		}
		return
	}

	response := bootstrapSuccessResponse{
		Message:     "Admin bootstrap completed.",
		TokenType:   result.TokenType,
		AccessToken: result.AccessToken,
		ExpiresIn:   result.ExpiresIn,
	}
	response.User.ID = result.User.ID
	response.User.Email = result.User.Email
	response.User.Role = result.User.Role

	writeJSON(w, http.StatusCreated, response)
}

func handleAuthLogin(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	var req bootstrapRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	result, err := deps.AuthService.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, coreauth.ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, apiError{Error: "invalid login payload"})
		case errors.Is(err, coreauth.ErrBootstrapRequired):
			writeJSON(w, http.StatusConflict, apiError{Error: "bootstrap required before login"})
		case errors.Is(err, coreauth.ErrInvalidCredentials):
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "invalid credentials"})
		default:
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "login failed"})
		}
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func sanitizeConfig(cfg config.Config, encryptionConfigured, jwtConfigured bool) sanitizedConfig {
	return sanitizedConfig{
		Server: sanitizedServer{
			Host: cfg.Server.Host,
			Port: cfg.Server.Port,
		},
		Metadata: sanitizedMetadata{
			Driver:             cfg.Metadata.Driver,
			SQLiteConfigured:   strings.TrimSpace(cfg.Metadata.SQLite.Path) != "",
			PostgresConfigured: strings.TrimSpace(cfg.Metadata.Postgres.DSN) != "",
		},
		Core: sanitizedCore{
			CacheTTL:                cfg.Core.CacheTTL,
			SweepInterval:           cfg.Core.Sweep,
			EncryptionKeyConfigured: encryptionConfigured,
		},
		Auth: sanitizedAuth{
			JWTSigningKeyConfigured: jwtConfigured,
		},
		Provisioning: sanitizedProvisioning{
			DefaultProvider: cfg.Provisioning.DefaultProvider,
			Docker: sanitizedDocker{
				Enabled:       cfg.Provisioning.Docker.Enabled,
				Mode:          cfg.Provisioning.Docker.Mode,
				PostgresImage: cfg.Provisioning.Docker.PostgresImage,
				MongoImage:    cfg.Provisioning.Docker.MongoImage,
				Endpoint:      "[redacted]",
				HostAddress:   "[redacted]",
				NetworkName:   "[redacted]",
			},
		},
	}
}

func decodeJSONBody(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("invalid json payload")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("invalid json payload")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
