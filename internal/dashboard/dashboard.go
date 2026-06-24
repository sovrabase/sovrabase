// Package dashboard provides an embedded admin dashboard for the Sovrabase
// Control Plane. It serves a single-page HTML application at the root path.
package dashboard

import (
	"embed"
	"net/http"
)

//go:embed index.html style.css js/*.js
var content embed.FS

// Handler returns an http.Handler that serves the embedded dashboard.
func Handler() http.Handler {
	return http.FileServer(http.FS(content))
}
