// Package plugin provides the extension API for Sovrabase.
// Plugins can hook into record lifecycle events, auth events,
// storage events, and register custom HTTP routes.
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
	db      DatabaseAccessor
	storage StorageAccessor
}

// Router returns the chi router so plugins can register custom routes
// and middleware.
func (a *App) Router() chi.Router {
	return a.router
}

// DatabaseAccessor gives plugins read access to the database.
type DatabaseAccessor interface {
	Get(collection, id string) (map[string]interface{}, error)
	List(collection string) ([]map[string]interface{}, error)
}

// StorageAccessor gives plugins read access to storage.
type StorageAccessor interface {
	List(bucket, prefix string) ([]FileInfo, error)
}

// FileInfo mirrors the API's file metadata.
type FileInfo struct {
	Bucket      string `json:"bucket"`
	Path        string `json:"path"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}
