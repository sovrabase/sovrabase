package api

import (
	"html/template"
	"net/http"

	_ "github.com/ketsuna-org/sovrabase/docs" // registers swagger spec
	"github.com/swaggo/swag"
)

const redocTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Sovrabase API Reference</title>
  <style>body{margin:0;padding:0}</style>
</head>
<body>
  <div id="redoc"></div>
  <script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
  <script>
    Redoc.init(
      {{.SpecURL}},
      {
        scrollYOffset: 0,
        hideDownloadButton: false,
        expandResponses: "200,201",
        nativeScrollbars: true,
        theme: {
          colors: {
            primary: { main: '#5b5bff' },
            success: { main: '#00c853' },
            error: { main: '#ff5252' },
            text: { primary: '#e0e0e0', secondary: '#5c5c66' },
            http: { get: '#61affe', post: '#49cc90', put: '#fca130', delete: '#f93e3e' },
          },
          typography: {
            fontFamily: '-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif',
            fontSize: '14px',
            headings: { fontFamily: 'inherit' },
            code: { fontFamily: '"JetBrains Mono", "Fira Code", monospace', fontSize: '13px' },
          },
          sidebar: { backgroundColor: '#0d0d0f', textColor: '#e0e0e0' },
          rightPanel: { backgroundColor: '#141416' },
        },
      }
    );
  </script>
</body>
</html>`

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("redoc").Parse(redocTemplate))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := map[string]interface{}{
		"SpecURL": "/docs/swagger.json",
	}
	tmpl.Execute(w, data)
}

func (s *Server) handleSwaggerJSON(w http.ResponseWriter, r *http.Request) {
	spec, err := swag.ReadDoc()
	if err != nil {
		writeError(w, http.StatusNotFound, "swagger spec not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(spec))
}
