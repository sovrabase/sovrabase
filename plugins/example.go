package statusplugin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ketsuna-org/sovrabase/internal/plugin"
)

// StatusPlugin demonstrates the Sovrabase plugin system.
type StatusPlugin struct{}

func (p *StatusPlugin) Name() string { return "status-plugin" }

func (p *StatusPlugin) Register(app *plugin.App) error {
	// ── Record hooks ──────────────────────────────────────────────
	app.OnRecordCreate("*").Do(func(e *plugin.RecordEvent) error {
		if _, ok := e.Record["status"]; !ok {
			e.Record["status"] = "draft"
		}
		return nil
	})

	app.OnRecordUpdate("posts").Do(func(e *plugin.RecordEvent) error {
		if e.OldRecord != nil && e.OldRecord["locked"] == true {
			return nil // real plugin would return fmt.Errorf("record is locked")
		}
		return nil
	})

	// ── Auth hooks ────────────────────────────────────────────────
	app.OnAuthSignUp().Do(func(e *plugin.AuthEvent) {
		slog.Info("new signup", "email", e.Email)
	})

	app.OnAuthOAuth().Do(func(e *plugin.AuthEvent) {
		slog.Info("oauth login", "provider", e.Provider, "email", e.Email)
	})

	// ── Realtime hooks ────────────────────────────────────────────
	app.OnRealtimeMessage("*").Do(func(e *plugin.RealtimeEvent) error {
		// Strip sensitive fields from realtime broadcasts
		delete(e.Data, "password")
		delete(e.Data, "email")
		return nil
	})

	// ── Email hooks ───────────────────────────────────────────────
	app.OnEmailSend().Do(func(e *plugin.EmailEvent) error {
		slog.Info("email sent", "to", e.To, "subject", e.Subject)
		return nil
	})

	// ── Lifecycle hooks ───────────────────────────────────────────
	app.OnServe().Do(func(router chi.Router) {
		router.Get("/plugin-demo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"plugin":"status-plugin","status":"active"}`))
		})
	})

	app.OnTerminate().Do(func() {
		slog.Info("status-plugin: shutting down")
	})

	// ── Log hooks ─────────────────────────────────────────────────
	app.OnLog().Do(func(e *plugin.LogEvent) {
		// Forward errors to external monitoring
		if e.Level == plugin.LogError {
			slog.Error("plugin intercepted error", "msg", e.Message)
		}
	})

	slog.Info("StatusPlugin: registered")
	return nil
}

var Plugin = &StatusPlugin{}
