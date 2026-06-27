// Package dashboard provides an embedded admin dashboard for the Sovrabase
// Control Plane. It serves a React SPA built with Vite at ../frontend/.
//
// Build: cd ../frontend && npm run build && cp -r dist ../internal/dashboard/
package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist/*
var content embed.FS

// Handler returns an http.Handler that serves the embedded dashboard.
func Handler() http.Handler {
	sub, err := fs.Sub(content, "dist")
	if err != nil {
		panic("dashboard: embedded dist/ directory not found — run: cd frontend && npm run build && cp -r dist ../internal/dashboard/")
	}
	return http.FileServer(http.FS(sub))
}
