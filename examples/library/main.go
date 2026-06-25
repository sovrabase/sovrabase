// Example: embed Sovrabase as a library with custom routes and DB access.
//
//	go run examples/library/main.go
package main

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ketsuna-org/sovrabase"
	"github.com/ketsuna-org/sovrabase/plugin"
)

func main() {
	app := sovrabase.New(sovrabase.Config{
		DataDir:   "./example-data",
		JWTSecret: "dev-secret-do-not-use-in-production",
	})
	app.Plugins().RegisterPlugin("my-custom-app")

	// ── Record hooks ────────────────────────────────────────────────
	app.Plugins().OnRecordCreate("*").Do(func(e *plugin.RecordEvent) error {
		if _, ok := e.Record["status"]; !ok {
			e.Record["status"] = "draft"
		}
		return nil
	})

	// ── Custom route with DB access ─────────────────────────────────
	app.Plugins().OnServe().Do(func(router chi.Router) {
		router.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
			posts, _ := app.Plugins().DB().List("posts")
			users, _ := app.Plugins().DB().List("users")

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"posts": len(posts),
				"users": len(users),
			})
		})

		slog.Info("Custom /stats route registered")
	})

	slog.Info("Starting on http://localhost:6070")
	slog.Info("Open http://localhost:6070/docs for API reference")
	slog.Info("Try http://localhost:6070/stats for custom endpoint")

	if err := app.Serve(); err != nil {
		slog.Error("server failed", "error", err)
	}
}
