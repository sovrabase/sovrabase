# Per-Project OAuth2 Provider Configuration

## Goal
Allow each sovrabase project to configure its own OAuth2 providers (Google, GitHub, Discord, or any OAuth2-compliant provider) via the admin API.

## Current State
- OAuth provider code (Google, GitHub) exists in `internal/auth/oauth.go` but is NEVER registered
- Projects have no OAuth configuration field
- `GetProjectEnv()` creates auth services but never calls `RegisterOAuthProvider()`
- No admin API to manage per-project OAuth config

## Design

### Provider Config Model
```go
type OAuthProviderConfig struct {
    Name          string   `json:"name"`          // "google", "github", "discord", ...
    ClientID      string   `json:"client_id"`
    ClientSecret  string   `json:"client_secret"`
    RedirectURL   string   `json:"redirect_url"`   // full callback URL
    AuthURL       string   `json:"auth_url"`        // OAuth authorize endpoint
    TokenURL      string   `json:"token_url"`       // OAuth token endpoint
    UserInfoURL   string   `json:"userinfo_url"`    // user profile endpoint
    Scopes        []string `json:"scopes"`          // ["email", "profile"]
    EmailField    string   `json:"email_field"`     // JSON field for email in userinfo response (e.g. "email")
    NameField     string   `json:"name_field"`      // JSON field for name
    AvatarField   string   `json:"avatar_field"`    // JSON field for avatar URL
    IDField       string   `json:"id_field"`        // JSON field for provider user ID
}
```

### Generic Provider
A `GenericOAuthProvider` implementing `auth.OAuthProvider` that reads from config — works with ANY OAuth2 provider.

### Project Schema Update
Add `OAuthProviders []OAuthProviderConfig` to the `Project` struct (persisted in Pebble).

### AuthService Wiring
In `GetProjectEnv()`, iterate project's `OAuthProviders` and register each via `RegisterOAuthProvider()`.

### Admin API
- `GET /admin/projects/{id}/auth/providers` — list configured providers (masked secrets)
- `PUT /admin/projects/{id}/auth/providers` — set all providers at once (replace)

## Implementation Tasks

### Task 1: Add GenericOAuthProvider to internal/auth/oauth.go
- Add `OAuthProviderConfig` struct
- Add `GenericOAuthProvider` implementing `OAuthProvider` interface
- Add `NewGenericOAuthProvider(config OAuthProviderConfig) (*GenericOAuthProvider, error)`
- Provider uses `golang.org/x/oauth2` for the OAuth flow and raw HTTP for userinfo

### Task 2: Update Project struct + GetProjectEnv wiring
- Add `OAuthProviders []auth.OAuthProviderConfig` to `Project` struct in `internal/tenant/manager.go`
- In `GetProjectEnv()`, iterate `proj.OAuthProviders` and register each provider
- Existing Google/GitHub hardcoded providers are kept but made optional (generic provider covers them too)

### Task 3: Add admin API endpoints for OAuth providers
- `GET /admin/projects/{id}/auth/providers` — returns list with masked client_secret
- `PUT /admin/projects/{id}/auth/providers` — replaces all providers
- Register routes in `RegisterRoutes()`

### Task 4: Verify build
- `go build ./...` and `go test ./...`
