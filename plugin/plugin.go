// Package plugin provides the extension API for Sovrabase.
// Plugins can hook into record lifecycle events, auth events,
// storage events, register custom HTTP routes, and access the database.
package plugin

import "github.com/go-chi/chi/v5"

// Plugin is the interface that all Sovrabase plugins must implement.
type Plugin interface {
	// Name returns a unique identifier for the plugin.
	Name() string
	// Register is called once at startup. Use the App to bind hooks.
	Register(app *App) error
}

// App exposes the plugin API to plugins.
type App struct {
	router  chi.Router
	manager *HookManager
	db      DB
	storage Storage
}

// NewApp creates a new App with the given hook manager.
// Use SetRouter, SetDB, SetStorage to configure additional accessors.
func NewApp(manager *HookManager) *App {
	return &App{manager: manager}
}

// SetRouter sets the chi router (available after server creation).
func (a *App) SetRouter(router chi.Router) {
	a.router = router
}

// SetDB sets the database accessor for plugins.
func (a *App) SetDB(db DB) {
	a.db = db
}

// SetStorage sets the storage accessor for plugins.
func (a *App) SetStorage(s Storage) {
	a.storage = s
}

// Router returns the chi router so plugins and library users can
// register custom routes and middleware.
//
//	app.Router().Get("/custom", func(w http.ResponseWriter, r *http.Request) {
//	    docs, _ := app.DB().List("posts")
//	    json.NewEncoder(w).Encode(docs)
//	})
func (a *App) Router() chi.Router {
	return a.router
}

// DB returns the database accessor. Plugins and library users can
// read and write documents from custom routes or hooks.
func (a *App) DB() DB {
	return a.db
}

// Storage returns the storage accessor for file operations.
func (a *App) Storage() Storage {
	return a.storage
}

// ─── Accessor interfaces ────────────────────────────────────────────

// DB provides read and write access to the document database.
// This is the same interface used by the API handlers — plugins
// get the same power as built-in endpoints.
type DB interface {
	Insert(collection, id string, doc map[string]interface{}) error
	Get(collection, id string) (map[string]interface{}, error)
	Update(collection, id string, doc map[string]interface{}) error
	Delete(collection, id string) error
	List(collection string) ([]map[string]interface{}, error)
	Query(collection string, filter map[string]interface{}, projection []string) ([]map[string]interface{}, error)
	Count(collection string) (int64, error)
	Search(collection string, query string, fields []string, limit int) ([]map[string]interface{}, error)
}

// Storage provides read and write access to the file storage.
type Storage interface {
	Upload(bucket, path string, contentType string, size int64) (*FileInfo, error)
	List(bucket, prefix string) ([]FileInfo, error)
	Delete(bucket, path string) error
}

// FileInfo mirrors the API's file metadata.
type FileInfo struct {
	Bucket      string `json:"bucket"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}
