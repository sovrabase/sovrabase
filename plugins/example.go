package statusplugin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ketsuna-org/sovrabase/internal/plugin"
)

// StatusPlugin is an example plugin that sets default status on new records.
type StatusPlugin struct{}

func (p *StatusPlugin) Name() string { return "status-plugin" }

func (p *StatusPlugin) Register(app *plugin.App) error {
	// Auto-set status=draft on new records.
	app.OnRecordCreate("*").Do(func(e *plugin.RecordEvent) error {
		if _, ok := e.Record["status"]; !ok {
			e.Record["status"] = "draft"
		}
		return nil
	})

	// Custom route.
	app.OnServe().Do(func(router chi.Router) {
		router.Get("/plugin-demo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"plugin":"status-plugin","status":"active"}`))
		})
	})

	slog.Info("StatusPlugin: registered")
	return nil
}

var Plugin = &StatusPlugin{}
