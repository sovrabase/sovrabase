package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/auth"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
	"github.com/ketsuna-org/sovrabase/plugin"
)

func TestAdminMCPEndToEnd(t *testing.T) {
	dir := t.TempDir()
	pm, err := tenant.NewProjectManager(dir, &config.Config{JWTSecret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	defer pm.Close()
	proj, err := pm.CreateProject("mcp-project", "owner-1")
	if err != nil {
		t.Fatal(err)
	}
	env, err := pm.GetProjectEnv(proj.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := env.Engine.CreateCollection("posts"); err != nil {
		t.Fatal(err)
	}

	admin := NewAdminServer(pm, &config.Config{DataDir: dir, SessionDuration: time.Hour}, "admin_secret", "admin@example.com", "pw")
	mux := http.NewServeMux()
	admin.RegisterRoutes(mux)
	adminTok, err := auth.GenerateAccessToken(&auth.User{ID: "admin", Email: "admin@example.com", Role: auth.RoleAdmin}, "admin_secret", time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	globalDB, _ := db.NewEngine(filepath.Join(dir, "global_db"))
	defer globalDB.Close()
	globalStorage, _ := storage.NewLocalDriver(filepath.Join(dir, "global_storage"), "")
	srv := NewServer(&Config{AllowOrigins: "*", JWTSecret: "g", DataDir: dir}, globalDB,
		WrapAuthService(auth.NewService("g", auth.NewInMemoryUserStore())), WrapStorageDriver(globalStorage), pm, plugin.NewHookManager())

	adminDo := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/admin/projects/"+proj.ID+path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+adminTok)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		return w
	}
	mcpDo := func(tok, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()
		srv.Handler().ServeHTTP(w, req)
		return w
	}

	// Default config: disabled, all tools enabled.
	w := adminDo("GET", "/mcp", "")
	var cfg struct {
		Enabled       bool     `json:"enabled"`
		Endpoint      string   `json:"endpoint"`
		DisabledTools []string `json:"disabled_tools"`
		Tools         []struct {
			Name    string `json:"name"`
			Scope   string `json:"scope"`
			Write   bool   `json:"write"`
			Enabled bool   `json:"enabled"`
		} `json:"tools"`
	}
	if w.Code != 200 {
		t.Fatalf("GET mcp: %d %s", w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if cfg.Enabled || cfg.Endpoint != "/mcp" || cfg.DisabledTools == nil || len(cfg.Tools) != 14 {
		t.Fatalf("default config: %s", w.Body)
	}

	// Create token.
	w = adminDo("POST", "/mcp/tokens", `{"name":"laptop","scopes":["read","write"]}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create token: %d %s", w.Code, w.Body)
	}
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	plain, _ := created["token"].(string)
	if !strings.HasPrefix(plain, "sbmcp_") || created["prefix"] != plain[:12] || created["last_used_at"] != nil {
		t.Fatalf("created token: %s", w.Body)
	}
	if _, ok := created["last_used_at"]; !ok {
		t.Fatalf("last_used_at must be present (null)")
	}

	// Disabled project -> 403.
	if w := mcpDo(plain, `{"jsonrpc":"2.0","id":1,"method":"ping"}`); w.Code != http.StatusForbidden {
		t.Fatalf("disabled: %d %s", w.Code, w.Body)
	}

	w = adminDo("PUT", "/mcp", `{"enabled":true,"disabled_tools":["delete_record"]}`)
	if w.Code != 200 {
		t.Fatalf("PUT mcp: %d %s", w.Code, w.Body)
	}
	json.Unmarshal(w.Body.Bytes(), &cfg)
	if !cfg.Enabled || len(cfg.DisabledTools) != 1 {
		t.Fatalf("PUT result: %s", w.Body)
	}
	if w := adminDo("PUT", "/mcp", `{"enabled":true,"disabled_tools":["nope"]}`); w.Code != 400 {
		t.Fatalf("unknown tool should be 400, got %d", w.Code)
	}

	w = mcpDo(plain, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"insert_record","arguments":{"collection":"posts","id":"p1","record":{"title":"hello"}}}}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), `"isError":true`) {
		t.Fatalf("insert: %d %s", w.Code, w.Body)
	}
	w = mcpDo(plain, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"query_records","arguments":{"collection":"posts","filter":{"title":"hello"}}}}`)
	if !strings.Contains(w.Body.String(), `p1`) {
		t.Fatalf("query: %s", w.Body)
	}
	w = mcpDo(plain, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"describe_collection","arguments":{"collection":"posts"}}}`)
	if !strings.Contains(w.Body.String(), `title`) || strings.Contains(w.Body.String(), `"isError":true`) {
		t.Fatalf("describe: %s", w.Body)
	}

	// Call log.
	w = adminDo("GET", "/mcp/calls?limit=2", "")
	var calls []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &calls)
	if len(calls) != 2 || calls[0]["tool"] != "describe_collection" || calls[0]["token_name"] != "laptop" || calls[0]["ok"] != true {
		t.Fatalf("calls: %s", w.Body)
	}

	// Token list shows last use, never the secret.
	w = adminDo("GET", "/mcp/tokens", "")
	if strings.Contains(w.Body.String(), plain) || strings.Contains(w.Body.String(), `"hash"`) {
		t.Fatalf("token list leaks secret: %s", w.Body)
	}
	var toks []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &toks)
	if len(toks) != 1 || toks[0]["last_used_at"] == nil {
		t.Fatalf("tokens: %s", w.Body)
	}

	// Revoke.
	if w := adminDo("DELETE", "/mcp/tokens/"+toks[0]["id"].(string), ""); w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := mcpDo(plain, `{"jsonrpc":"2.0","id":1,"method":"ping"}`); w.Code != http.StatusUnauthorized {
		t.Fatalf("revoked: %d", w.Code)
	}
}
