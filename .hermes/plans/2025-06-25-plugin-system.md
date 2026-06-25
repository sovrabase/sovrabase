# Sovrabase Plugin System — Go Extension API

## Goal
Allow users to extend Sovrabase with custom Go plugins, PocketBase-style:
hooks on records/auth/storage + custom routes + CLI commands.

## Architecture

### 1. Plugin interface (`internal/plugin/plugin.go`)
```go
type Plugin interface {
    Name() string
    Register(app *App) error
}
```

### 2. App facade (`internal/plugin/app.go`)
Exposes safe access to sovrabase internals:
```go
type App struct {
    hooks     *HookManager
    router    chi.Router
    db        DatabaseService
    auth      AuthService
    storage   StorageService
    config    *config.Config
}
func (a *App) OnRecordCreate(collection string) *HookBuilder
func (a *App) OnRecordUpdate(collection string) *HookBuilder
func (a *App) OnRecordDelete(collection string) *HookBuilder
func (a *App) OnAuthSignUp() *HookBuilder
func (a *App) OnAuthSignIn() *HookBuilder
func (a *App) OnServe() *ServeHookBuilder
func (a *App) Router() chi.Router
```

### 3. Hook types
- **RecordHook**: `func(e *RecordEvent) error` — can read/modify record, abort with error
- **AuthHook**: `func(e *AuthEvent) error` — can read user info, abort with error
- **ServeHook**: `func(router chi.Router)` — register custom routes/middleware

### 4. HookBuilder (fluent API)
```go
app.OnRecordCreate("posts").Do(func(e *RecordEvent) error {
    if e.Record["status"] == nil {
        e.Record["status"] = "draft"
    }
    return nil
})
```

### 5. Integration points
| Hook | Integration point | File |
|---|---|---|
| OnRecordCreate | handleInsert() before DB write | handlers.go |
| OnRecordUpdate | handleUpdate() before DB write | handlers.go |
| OnRecordDelete | handleDelete() before DB write | handlers.go |
| OnAuthSignUp | handleSignUp() after validation | handlers.go |
| OnAuthSignIn | handleSignIn() after credentials check | handlers.go |
| OnServe | NewServer() before ListenAndServe | server.go |
| OnStorageUpload | handleUpload() before storage write | handlers.go |
| OnStorageDelete | handleStorageDelete() before delete | handlers.go |

### 6. Plugin loading
- Config: `plugins_dir: ./plugins` in config.yaml
- At startup, scan `plugins_dir/*.go` — compile via `go build -buildmode=plugin`
- Alternative (simpler for v1): register programmatically in main.go

### 7. Files to create
```
internal/plugin/
  plugin.go      — Plugin interface
  app.go         — App facade, HookBuilder, HookManager
  events.go      — RecordEvent, AuthEvent, StorageEvent
  hooks.go       — Hook execution chains
```

### 8. Files to modify
- `internal/api/handlers.go` — call hooks in insert/update/delete handlers
- `internal/api/server.go` — wire App, expose Router
- `cmd/sovrabase/main.go` — load plugins from config dir
- `internal/config/config.go` — add PluginsDir field

## Implementation order
1. Create `internal/plugin/` package (plugin.go, events.go, hooks.go, app.go)
2. Wire hooks into handlers.go
3. Wire Serve hook into server.go
4. Add config support for plugins_dir
5. Create example plugin in `plugins/example.go`
6. Test end-to-end
7. Update docs

## Out of scope (v2)
- JavaScript plugin support (goja)
- Hot reload
- Plugin marketplace
- CLI command registration (cobra)
