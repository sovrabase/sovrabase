package httpapi

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/config"
	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func TestPostConfigStatuses(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		bootstrap    coreauth.AuthResult
		bootstrapErr error
		wantStatus   int
	}{
		{
			name: "created",
			body: `{"email":"admin@example.com","password":"very-strong-password"}`,
			bootstrap: coreauth.AuthResult{
				TokenType:   "Bearer",
				AccessToken: "token",
				ExpiresIn:   86400,
				User:        coreauth.PublicUser{ID: "u1", Email: "admin@example.com", Role: coreauth.UserRoleAdmin},
			},
			wantStatus: http.StatusCreated,
		},
		{
			name:         "conflict",
			body:         `{"email":"admin@example.com","password":"very-strong-password"}`,
			bootstrapErr: coreauth.ErrBootstrapAlreadyDone,
			wantStatus:   http.StatusConflict,
		},
		{
			name:       "bad_request",
			body:       `{"email":`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := &fakeAuthService{
				bootstrapRequired: true,
				bootstrapResult:   tt.bootstrap,
				bootstrapErr:      tt.bootstrapErr,
			}

			mux := http.NewServeMux()
			err := RegisterRoutes(mux, Dependencies{
				Config:                  config.Default(),
				AuthService:             authSvc,
				MetadataPinger:          fakePinger{err: nil},
				Logger:                  &fakeLogger{},
				EncryptionKeyConfigured: true,
				JWTSigningKeyConfigured: true,
			})
			if err != nil {
				t.Fatalf("RegisterRoutes() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/config", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("POST /config status = %d, want %d (body=%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestPostAuthLoginStatuses(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		login      coreauth.AuthResult
		loginErr   error
		wantStatus int
	}{
		{
			name: "ok",
			body: `{"email":"admin@example.com","password":"very-strong-password"}`,
			login: coreauth.AuthResult{
				TokenType:   "Bearer",
				AccessToken: "token",
				ExpiresIn:   86400,
				User:        coreauth.PublicUser{ID: "u1", Email: "admin@example.com", Role: coreauth.UserRoleAdmin},
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "unauthorized",
			body:       `{"email":"admin@example.com","password":"very-strong-password"}`,
			loginErr:   coreauth.ErrInvalidCredentials,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "bootstrap_required",
			body:       `{"email":"admin@example.com","password":"very-strong-password"}`,
			loginErr:   coreauth.ErrBootstrapRequired,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "bad_request",
			body:       `{"email":`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "internal_error",
			body:       `{"email":"admin@example.com","password":"very-strong-password"}`,
			loginErr:   errors.New("db is down"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authSvc := &fakeAuthService{
				bootstrapRequired: false,
				loginResult:       tt.login,
				loginErr:          tt.loginErr,
			}

			mux := http.NewServeMux()
			err := RegisterRoutes(mux, Dependencies{
				Config:                  config.Default(),
				AuthService:             authSvc,
				MetadataPinger:          fakePinger{err: nil},
				Logger:                  &fakeLogger{},
				EncryptionKeyConfigured: true,
				JWTSigningKeyConfigured: true,
			})
			if err != nil {
				t.Fatalf("RegisterRoutes() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("POST /auth/login status = %d, want %d (body=%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}
