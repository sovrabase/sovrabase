package httpapi

import (
	"net/http"
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/config"
)

func TestRegisterRoutesDependencies(t *testing.T) {
	tests := []struct {
		name    string
		mux     *http.ServeMux
		deps    Dependencies
		wantErr string
	}{
		{
			name:    "nil mux",
			mux:     nil,
			deps:    Dependencies{},
			wantErr: "mux is required",
		},
		{
			name: "nil auth service",
			mux:  http.NewServeMux(),
			deps: Dependencies{
				MetadataPinger: fakePinger{},
				Logger:         &fakeLogger{},
			},
			wantErr: "auth service is required",
		},
		{
			name: "nil metadata pinger",
			mux:  http.NewServeMux(),
			deps: Dependencies{
				AuthService: &fakeAuthService{},
				Logger:      &fakeLogger{},
			},
			wantErr: "metadata pinger is required",
		},
		{
			name: "nil logger",
			mux:  http.NewServeMux(),
			deps: Dependencies{
				AuthService:    &fakeAuthService{},
				MetadataPinger: fakePinger{},
			},
			wantErr: "logger is required",
		},
		{
			name: "valid deps",
			mux:  http.NewServeMux(),
			deps: Dependencies{
				Config:                  config.Default(),
				AuthService:             &fakeAuthService{},
				MetadataPinger:          fakePinger{},
				Logger:                  &fakeLogger{},
				EncryptionKeyConfigured: true,
				JWTSigningKeyConfigured: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RegisterRoutes(tt.mux, tt.deps)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("RegisterRoutes() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("RegisterRoutes() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("RegisterRoutes() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
