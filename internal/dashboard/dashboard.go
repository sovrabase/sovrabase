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

//go:embed dist
var content embed.FS

// Handler returns an http.Handler that serves the embedded React SPA
// with client-side routing fallback.
func Handler() http.Handler {
	sub, err := fs.Sub(content, "dist")
	if err != nil {
		panic("dashboard: embedded dist/ not found")
	}

	fileServer := http.FileServerFS(sub)

	// Read index.html once for SPA fallback
	indexHTML, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		panic("dashboard: embedded dist/index.html not found")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the file directly
		path := r.URL.Path
		f, err := sub.Open(path[1:]) // strip leading /
		if err == nil {
			f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA fallback: serve index.html
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(indexHTML)
	})
}
