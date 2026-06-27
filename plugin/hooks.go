package plugin

import "github.com/go-chi/chi/v5"

// HookManager stores and dispatches all registered hooks.
type HookManager struct {
	recordHooks     map[string][]RecordHookFunc
	collectionHooks map[string][]CollectionHookFunc
	authHooks       map[string][]AuthHookFunc
	storageHooks    map[string][]StorageHookFunc
	realtimeHooks   map[string][]RealtimeHookFunc
	emailHooks      []EmailHookFunc
	terminateHooks  []TerminateHookFunc
	logHooks        []LogHookFunc
	serveHooks      []ServeHookFunc
}

// NewHookManager creates an empty hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		recordHooks:     make(map[string][]RecordHookFunc),
		collectionHooks: make(map[string][]CollectionHookFunc),
		authHooks:       make(map[string][]AuthHookFunc),
		storageHooks:    make(map[string][]StorageHookFunc),
		realtimeHooks:   make(map[string][]RealtimeHookFunc),
	}
}

// ─── Hook function types ──────────────────────────────────────────────

// RecordHookFunc is called on record create/update/delete.
// Return a non-nil error to abort the operation.
type RecordHookFunc func(e *RecordEvent) error

// CollectionHookFunc is called on admin collection create/update/delete.
// Return a non-nil error to abort the operation.
type CollectionHookFunc func(e *CollectionEvent) error

// AuthHookFunc is called on signup/signin.
// Set e.Abort=true to reject the auth attempt.
type AuthHookFunc func(e *AuthEvent)

// StorageHookFunc is called on storage upload/delete.
// Return a non-nil error to abort the operation.
type StorageHookFunc func(e *StorageEvent) error

// RealtimeHookFunc is called before a realtime event is broadcast.
// Return a non-nil error to suppress the broadcast.
// Modify e.Data to transform the payload.
type RealtimeHookFunc func(e *RealtimeEvent) error

// EmailHookFunc is called when an email is about to be sent.
// Modify e to change recipient, subject, body, etc.
// Return a non-nil error to abort sending.
type EmailHookFunc func(e *EmailEvent) error

// TerminateHookFunc is called when the server is shutting down.
type TerminateHookFunc func()

// LogHookFunc is called for every log entry emitted by the server.
type LogHookFunc func(e *LogEvent)

// ServeHookFunc is called when the server starts.
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

// OnCollectionCreate registers a hook for collection creation (admin).
func (a *App) OnCollectionCreate() *HookBuilder[CollectionHookFunc] {
	return &HookBuilder[CollectionHookFunc]{
		register: func(fn CollectionHookFunc) {
			a.manager.collectionHooks["create"] = append(a.manager.collectionHooks["create"], fn)
		},
	}
}

// OnCollectionUpdate registers a hook for collection updates (admin).
func (a *App) OnCollectionUpdate() *HookBuilder[CollectionHookFunc] {
	return &HookBuilder[CollectionHookFunc]{
		register: func(fn CollectionHookFunc) {
			a.manager.collectionHooks["update"] = append(a.manager.collectionHooks["update"], fn)
		},
	}
}

// OnCollectionDelete registers a hook for collection deletion (admin).
func (a *App) OnCollectionDelete() *HookBuilder[CollectionHookFunc] {
	return &HookBuilder[CollectionHookFunc]{
		register: func(fn CollectionHookFunc) {
			a.manager.collectionHooks["delete"] = append(a.manager.collectionHooks["delete"], fn)
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

// OnAuthRefresh registers a hook for token refresh.
func (a *App) OnAuthRefresh() *HookBuilder[AuthHookFunc] {
	return &HookBuilder[AuthHookFunc]{
		register: func(fn AuthHookFunc) {
			a.manager.authHooks["refresh"] = append(a.manager.authHooks["refresh"], fn)
		},
	}
}

// OnAuthOAuth registers a hook for OAuth login/callback.
func (a *App) OnAuthOAuth() *HookBuilder[AuthHookFunc] {
	return &HookBuilder[AuthHookFunc]{
		register: func(fn AuthHookFunc) {
			a.manager.authHooks["oauth"] = append(a.manager.authHooks["oauth"], fn)
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

// OnStorageDownload registers a hook for file downloads (read-only, cannot abort).
func (a *App) OnStorageDownload() *HookBuilder[StorageHookFunc] {
	return &HookBuilder[StorageHookFunc]{
		register: func(fn StorageHookFunc) {
			a.manager.storageHooks["download"] = append(a.manager.storageHooks["download"], fn)
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

// OnRealtimeMessage registers a hook for realtime messages before broadcast.
// Use collection="*" to match all collections.
func (a *App) OnRealtimeMessage(collection string) *HookBuilder[RealtimeHookFunc] {
	return &HookBuilder[RealtimeHookFunc]{
		register: func(fn RealtimeHookFunc) {
			a.manager.realtimeHooks[collection] = append(a.manager.realtimeHooks[collection], fn)
		},
	}
}

// OnEmailSend registers a hook for outgoing emails.
func (a *App) OnEmailSend() *HookBuilder[EmailHookFunc] {
	return &HookBuilder[EmailHookFunc]{
		register: func(fn EmailHookFunc) {
			a.manager.emailHooks = append(a.manager.emailHooks, fn)
		},
	}
}

// OnTerminate registers a hook called during server shutdown.
func (a *App) OnTerminate() *HookBuilder[TerminateHookFunc] {
	return &HookBuilder[TerminateHookFunc]{
		register: func(fn TerminateHookFunc) {
			a.manager.terminateHooks = append(a.manager.terminateHooks, fn)
		},
	}
}

// OnLog registers a hook for log events.
func (a *App) OnLog() *HookBuilder[LogHookFunc] {
	return &HookBuilder[LogHookFunc]{
		register: func(fn LogHookFunc) {
			a.manager.logHooks = append(a.manager.logHooks, fn)
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

// RunRecordHooks executes all matching record hooks.
func (m *HookManager) RunRecordHooks(collection, action string, e *RecordEvent) error {
	key := collection + ":" + action
	for _, fn := range m.recordHooks[key] {
		if err := fn(e); err != nil {
			return err
		}
	}
	wildKey := "*:" + action
	for _, fn := range m.recordHooks[wildKey] {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// RunCollectionHooks executes all collection hooks for the given action.
func (m *HookManager) RunCollectionHooks(action string, e *CollectionEvent) error {
	for _, fn := range m.collectionHooks[action] {
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

// RunRealtimeHooks executes all realtime hooks matching the collection.
// Returns true if the event should be broadcast (no hook suppressed it).
func (m *HookManager) RunRealtimeHooks(collection string, e *RealtimeEvent) bool {
	// Exact match
	for _, fn := range m.realtimeHooks[collection] {
		if err := fn(e); err != nil {
			return false
		}
	}
	// Wildcard
	for _, fn := range m.realtimeHooks["*"] {
		if err := fn(e); err != nil {
			return false
		}
	}
	return true
}

// RunEmailHooks executes all email hooks. Returns error if any hook aborts sending.
func (m *HookManager) RunEmailHooks(e *EmailEvent) error {
	for _, fn := range m.emailHooks {
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

// RunTerminateHooks executes all terminate hooks.
func (m *HookManager) RunTerminateHooks() {
	for _, fn := range m.terminateHooks {
		fn()
	}
}

// RunLogHooks executes all log hooks.
func (m *HookManager) RunLogHooks(e *LogEvent) {
	for _, fn := range m.logHooks {
		fn(e)
	}
}

// RunServeHooks executes all serve hooks with the router.
func (m *HookManager) RunServeHooks(router chi.Router) {
	for _, fn := range m.serveHooks {
		fn(router)
	}
}

// ─── Introspection ──────────────────────────────────────────────────

// HookInfo describes a single registered hook.
type HookInfo struct {
	Type       string `json:"type"`       // "record", "auth", "realtime", "storage", "email", "serve", "terminate", "log", "collection"
	Action     string `json:"action"`     // "create", "update", "signup", etc.
	Collection string `json:"collection"` // collection name, empty for non-collection hooks
	Count      int    `json:"count"`      // number of registered callbacks
}

// RouteInfo describes a registered HTTP route.
type RouteInfo struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

// PluginInfo is the full introspection payload returned by the admin API.
type PluginInfo struct {
	Plugins []string   `json:"plugins"`
	Hooks   []HookInfo `json:"hooks"`
	Routes  []RouteInfo `json:"routes"`
}

// Info returns a summary of all registered hooks and plugins.
func (m *HookManager) Info() []HookInfo {
	hooks := []HookInfo{}

	for key, fns := range m.recordHooks {
		parts := splitKey(key) // "posts:create" → col="posts", action="create"
		hooks = append(hooks, HookInfo{
			Type:       "record",
			Action:     parts[1],
			Collection: parts[0],
			Count:      len(fns),
		})
	}
	for action, fns := range m.collectionHooks {
		hooks = append(hooks, HookInfo{
			Type:   "collection",
			Action: action,
			Count:  len(fns),
		})
	}
	for action, fns := range m.authHooks {
		hooks = append(hooks, HookInfo{
			Type:   "auth",
			Action: action,
			Count:  len(fns),
		})
	}
	for action, fns := range m.storageHooks {
		hooks = append(hooks, HookInfo{
			Type:   "storage",
			Action: action,
			Count:  len(fns),
		})
	}
	for col, fns := range m.realtimeHooks {
		hooks = append(hooks, HookInfo{
			Type:       "realtime",
			Collection: col,
			Count:      len(fns),
		})
	}
	if len(m.emailHooks) > 0 {
		hooks = append(hooks, HookInfo{Type: "email", Count: len(m.emailHooks)})
	}
	if len(m.terminateHooks) > 0 {
		hooks = append(hooks, HookInfo{Type: "terminate", Count: len(m.terminateHooks)})
	}
	if len(m.logHooks) > 0 {
		hooks = append(hooks, HookInfo{Type: "log", Count: len(m.logHooks)})
	}
	if len(m.serveHooks) > 0 {
		hooks = append(hooks, HookInfo{Type: "serve", Count: len(m.serveHooks)})
	}
	return hooks
}

func splitKey(key string) [2]string {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == ':' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, ""}
}

type HookBuilder[T any] struct {
	register func(T)
}

func (b *HookBuilder[T]) Do(fn T) {
	b.register(fn)
}
