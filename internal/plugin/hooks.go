package plugin

import "github.com/go-chi/chi/v5"

// HookManager stores and dispatches all registered hooks.
type HookManager struct {
	recordHooks map[string][]RecordHookFunc // key: "collection:create", "collection:update", etc.
	authHooks   map[string][]AuthHookFunc   // key: "signup", "signin"
	storageHooks map[string][]StorageHookFunc // key: "upload", "delete"
	serveHooks  []ServeHookFunc
}

// NewHookManager creates an empty hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		recordHooks:  make(map[string][]RecordHookFunc),
		authHooks:    make(map[string][]AuthHookFunc),
		storageHooks: make(map[string][]StorageHookFunc),
	}
}

// ─── Hook function types ──────────────────────────────────────────────

// RecordHookFunc is called on record create/update/delete.
// Return a non-nil error to abort the operation.
type RecordHookFunc func(e *RecordEvent) error

// AuthHookFunc is called on signup/signin.
// Set e.Abort=true to reject the auth attempt.
type AuthHookFunc func(e *AuthEvent)

// StorageHookFunc is called on storage upload/delete.
// Return a non-nil error to abort the operation.
type StorageHookFunc func(e *StorageEvent) error

// ServeHookFunc is called when the server starts.
// Plugins can register custom routes on the provided router.
type ServeHookFunc func(router chi.Router)

// ─── Hook registration ───────────────────────────────────────────────

// OnRecordCreate registers a hook for record creation on a collection.
// Use "*" to match all collections.
func (a *App) OnRecordCreate(collection string) *HookBuilder[RecordHookFunc] {
	return &HookBuilder[RecordHookFunc]{
		register: func(fn RecordHookFunc) {
			key := collection + ":create"
			a.manager.recordHooks[key] = append(a.manager.recordHooks[key], fn)
		},
	}
}

// OnRecordUpdate registers a hook for record updates on a collection.
func (a *App) OnRecordUpdate(collection string) *HookBuilder[RecordHookFunc] {
	return &HookBuilder[RecordHookFunc]{
		register: func(fn RecordHookFunc) {
			key := collection + ":update"
			a.manager.recordHooks[key] = append(a.manager.recordHooks[key], fn)
		},
	}
}

// OnRecordDelete registers a hook for record deletion on a collection.
func (a *App) OnRecordDelete(collection string) *HookBuilder[RecordHookFunc] {
	return &HookBuilder[RecordHookFunc]{
		register: func(fn RecordHookFunc) {
			key := collection + ":delete"
			a.manager.recordHooks[key] = append(a.manager.recordHooks[key], fn)
		},
	}
}

// OnAuthSignUp registers a hook for user signup.
func (a *App) OnAuthSignUp() *HookBuilder[AuthHookFunc] {
	return &HookBuilder[AuthHookFunc]{
		register: func(fn AuthHookFunc) {
			a.manager.authHooks["signup"] = append(a.manager.authHooks["signup"], fn)
		},
	}
}

// OnAuthSignIn registers a hook for user signin.
func (a *App) OnAuthSignIn() *HookBuilder[AuthHookFunc] {
	return &HookBuilder[AuthHookFunc]{
		register: func(fn AuthHookFunc) {
			a.manager.authHooks["signin"] = append(a.manager.authHooks["signin"], fn)
		},
	}
}

// OnStorageUpload registers a hook for file uploads.
func (a *App) OnStorageUpload() *HookBuilder[StorageHookFunc] {
	return &HookBuilder[StorageHookFunc]{
		register: func(fn StorageHookFunc) {
			a.manager.storageHooks["upload"] = append(a.manager.storageHooks["upload"], fn)
		},
	}
}

// OnStorageDelete registers a hook for file deletions.
func (a *App) OnStorageDelete() *HookBuilder[StorageHookFunc] {
	return &HookBuilder[StorageHookFunc]{
		register: func(fn StorageHookFunc) {
			a.manager.storageHooks["delete"] = append(a.manager.storageHooks["delete"], fn)
		},
	}
}

// OnServe registers a hook called when the server starts.
func (a *App) OnServe() *HookBuilder[ServeHookFunc] {
	return &HookBuilder[ServeHookFunc]{
		register: func(fn ServeHookFunc) {
			a.manager.serveHooks = append(a.manager.serveHooks, fn)
		},
	}
}

// ─── Hook execution ──────────────────────────────────────────────────

// RunRecordHooks executes all matching record hooks. Returns an error if any hook aborts.
func (m *HookManager) RunRecordHooks(collection, action string, e *RecordEvent) error {
	// Exact match first
	key := collection + ":" + action
	for _, fn := range m.recordHooks[key] {
		if err := fn(e); err != nil {
			return err
		}
	}
	// Wildcard match
	wildKey := "*:" + action
	for _, fn := range m.recordHooks[wildKey] {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// RunAuthHooks executes all auth hooks for the given action.
func (m *HookManager) RunAuthHooks(action string, e *AuthEvent) {
	for _, fn := range m.authHooks[action] {
		fn(e)
		if e.Abort {
			return
		}
	}
}

// RunStorageHooks executes all storage hooks for the given action.
func (m *HookManager) RunStorageHooks(action string, e *StorageEvent) error {
	for _, fn := range m.storageHooks[action] {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// RunServeHooks executes all serve hooks with the router.
func (m *HookManager) RunServeHooks(router chi.Router) {
	for _, fn := range m.serveHooks {
		fn(router)
	}
}

// ─── HookBuilder (fluent API) ────────────────────────────────────────

// HookBuilder provides a fluent API for registering hooks.
// Usage: app.OnRecordCreate("posts").Do(func(e *RecordEvent) error { ... })
type HookBuilder[T any] struct {
	register func(T)
}

// Do registers the hook function.
func (b *HookBuilder[T]) Do(fn T) {
	b.register(fn)
}
