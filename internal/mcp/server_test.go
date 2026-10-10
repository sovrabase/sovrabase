package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/cockroachdb/pebble"
)

// ── fakes ──────────────────────────────────────────────────────────────────

type fakeBackend struct {
	cols map[string]map[string]map[string]interface{}
}

func newFakeBackend() *fakeBackend {
	return &fakeBackend{cols: map[string]map[string]map[string]interface{}{
		"posts": {
			"p1": {"_id": "p1", "title": "b", "views": float64(10)},
			"p2": {"_id": "p2", "title": "a", "views": float64(5)},
		},
		"_users": {
			"u1": {"_id": "u1", "email": "x@y.z", "password_hash": "secret"},
		},
	}}
}

func (f *fakeBackend) ListCollections() ([]string, error) {
	out := []string{"__internal"}
	for k := range f.cols {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
func (f *fakeBackend) CountRecords(c string) (int64, error) { return int64(len(f.cols[c])), nil }
func (f *fakeBackend) CreateCollection(name string) error {
	if _, ok := f.cols[name]; ok {
		return errors.New("exists")
	}
	f.cols[name] = map[string]map[string]interface{}{}
	return nil
}
func (f *fakeBackend) QueryRecords(c string, filter map[string]interface{}, limit, offset int) ([]map[string]interface{}, error) {
	var out []map[string]interface{}
	ids := make([]string, 0)
	for id := range f.cols[c] {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		d := f.cols[c][id]
		match := true
		for k, v := range filter {
			if d[k] != v {
				match = false
			}
		}
		if match {
			cp := map[string]interface{}{}
			for k, v := range d {
				cp[k] = v
			}
			out = append(out, cp)
		}
	}
	if offset < len(out) {
		out = out[offset:]
	} else {
		out = nil
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (f *fakeBackend) GetRecord(c, id string) (map[string]interface{}, error) {
	d, ok := f.cols[c][id]
	if !ok {
		return nil, nil
	}
	cp := map[string]interface{}{}
	for k, v := range d {
		cp[k] = v
	}
	return cp, nil
}
func (f *fakeBackend) InsertRecord(c, id string, doc map[string]interface{}) error {
	if id == "" {
		id = "gen"
	}
	doc["_id"] = id
	f.cols[c][id] = doc
	return nil
}
func (f *fakeBackend) UpdateRecord(c, id string, doc map[string]interface{}) error {
	doc["_id"] = id
	f.cols[c][id] = doc
	return nil
}
func (f *fakeBackend) DeleteRecord(c, id string) error { delete(f.cols[c], id); return nil }
func (f *fakeBackend) ListIndexes(c string) ([]IndexInfo, error) {
	return []IndexInfo{{Field: "title", Type: "simple"}}, nil
}
func (f *fakeBackend) CreateIndex(c, field string, unique bool) error { return nil }
func (f *fakeBackend) GetRules(c string) (interface{}, error) {
	return map[string]interface{}{"enabled": false}, nil
}
func (f *fakeBackend) ListFunctions() ([]FunctionInfo, error) {
	return []FunctionInfo{{Name: "hello", Trigger: "http"}}, nil
}
func (f *fakeBackend) ListBuckets(ctx context.Context) ([]string, error) {
	return []string{"avatars"}, nil
}
func (f *fakeBackend) InvokeFunction(ctx context.Context, name string, p map[string]interface{}) (interface{}, error) {
	return map[string]interface{}{"status": "success"}, nil
}
func (f *fakeBackend) ListFiles(ctx context.Context, b, p string) (interface{}, error) {
	return []string{}, nil
}
func (f *fakeBackend) ReadLogs(max int) ([]map[string]interface{}, error) {
	return []map[string]interface{}{
		{"path": "/a", "status": float64(500)},
		{"path": "/b", "status": float64(404)},
		{"path": "/c", "status": float64(200)},
	}, nil
}

type fakeProvider struct {
	dbs     map[string]*pebble.DB
	backend *fakeBackend
}

func (p *fakeProvider) ProjectIDs() []string {
	ids := []string{}
	for id := range p.dbs {
		ids = append(ids, id)
	}
	return ids
}
func (p *fakeProvider) ProjectDB(id string) (*pebble.DB, error) {
	db, ok := p.dbs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return db, nil
}
func (p *fakeProvider) Backend(id string) (Backend, error) { return p.backend, nil }

func newTestServer(t *testing.T) (*Server, *fakeProvider) {
	t.Helper()
	db, err := pebble.Open(t.TempDir(), &pebble.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	p := &fakeProvider{dbs: map[string]*pebble.DB{"proj1": db}, backend: newFakeBackend()}
	return NewServer(p), p
}

func enable(t *testing.T, s *Server, disabled ...string) {
	t.Helper()
	st, err := s.Store("proj1")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetConfig(Config{Enabled: true, DisabledTools: disabled}); err != nil {
		t.Fatal(err)
	}
}

func token(t *testing.T, s *Server, scopes ...string) string {
	t.Helper()
	_, plain, err := s.CreateToken("proj1", "test", scopes)
	if err != nil {
		t.Fatal(err)
	}
	return plain
}

type rpcResult struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

func post(t *testing.T, s *Server, tok, body string) (*httptest.ResponseRecorder, rpcResult) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	var res rpcResult
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Fatalf("bad JSON response %q: %v", rec.Body.String(), err)
		}
	}
	return rec, res
}

func toolNames(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var r struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Fatal(err)
	}
	out := []string{}
	for _, tl := range r.Tools {
		out = append(out, tl.Name)
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestInitialize(t *testing.T) {
	s, _ := newTestServer(t)
	enable(t, s)
	tok := token(t, s)
	rec, res := post(t, s, tok, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`)
	if rec.Code != 200 || res.Error != nil {
		t.Fatalf("status %d err %+v", rec.Code, res.Error)
	}
	if string(res.ID) != "1" {
		t.Fatalf("id = %s", res.ID)
	}
	var r struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
		ServerInfo      struct{ Name string }      `json:"serverInfo"`
	}
	json.Unmarshal(res.Result, &r)
	if r.ProtocolVersion != "2025-03-26" {
		t.Errorf("protocolVersion = %q", r.ProtocolVersion)
	}
	if r.ServerInfo.Name != "sovrabase" {
		t.Errorf("serverInfo.name = %q", r.ServerInfo.Name)
	}
	if _, ok := r.Capabilities["tools"]; !ok {
		t.Errorf("missing tools capability")
	}

	// Unknown version falls back to the latest.
	_, res = post(t, s, tok, `{"jsonrpc":"2.0","id":2,"method":"initialize","params":{"protocolVersion":"1999-01-01"}}`)
	json.Unmarshal(res.Result, &r)
	if r.ProtocolVersion != ProtocolVersion {
		t.Errorf("fallback protocolVersion = %q", r.ProtocolVersion)
	}

	// initialized notification: 202, no body.
	rec, _ = post(t, s, tok, `{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	if rec.Code != http.StatusAccepted || rec.Body.Len() != 0 {
		t.Errorf("notification: status %d body %q", rec.Code, rec.Body.String())
	}

	_, res = post(t, s, tok, `{"jsonrpc":"2.0","id":"p","method":"ping"}`)
	if res.Error != nil || string(res.Result) != "{}" {
		t.Errorf("ping: %+v %s", res.Error, res.Result)
	}
}

func TestToolsListRespectsScopesAndDisabled(t *testing.T) {
	s, _ := newTestServer(t)
	enable(t, s, "get_record")

	readTok := token(t, s) // default: read
	_, res := post(t, s, readTok, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	names := toolNames(t, res.Result)
	for _, want := range []string{"list_collections", "describe_collection", "query_records", "list_buckets", "list_files"} {
		if !contains(names, want) {
			t.Errorf("read token missing %s (got %v)", want, names)
		}
	}
	for _, no := range []string{"get_record", "insert_record", "create_collection", "list_functions", "read_logs"} {
		if contains(names, no) {
			t.Errorf("read token should not see %s", no)
		}
	}

	allTok := token(t, s, "read", "write", "schema", "functions", "logs")
	_, res = post(t, s, allTok, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	names = toolNames(t, res.Result)
	if len(names) != len(Tools())-1 || contains(names, "get_record") {
		t.Errorf("all-scope token tools = %v", names)
	}

	// Disabled tools are rejected on call.
	_, res = post(t, s, allTok, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_record","arguments":{"collection":"posts","id":"p1"}}}`)
	if res.Error == nil || res.Error.Code != codeInvalidParams {
		t.Errorf("disabled tool call: %+v", res.Error)
	}
	// Missing scope gives an isError result.
	_, res = post(t, s, readTok, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"insert_record","arguments":{"collection":"posts","record":{}}}}`)
	if !strings.Contains(string(res.Result), `"isError":true`) {
		t.Errorf("scope check: %s", res.Result)
	}

	calls := s.Calls("proj1", 10)
	if len(calls) != 2 || calls[0].Tool != "insert_record" || calls[0].OK || calls[1].Tool != "get_record" {
		t.Errorf("calls = %+v", calls)
	}
}

func TestToolCalls(t *testing.T) {
	s, p := newTestServer(t)
	enable(t, s)
	tok := token(t, s, "read", "write", "logs")

	call := func(name, args string) map[string]interface{} {
		t.Helper()
		_, res := post(t, s, tok, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`)
		if res.Error != nil {
			t.Fatalf("%s: rpc error %+v", name, res.Error)
		}
		var r struct {
			Content []struct{ Type, Text string }
			IsError bool `json:"isError"`
		}
		json.Unmarshal(res.Result, &r)
		if len(r.Content) != 1 || r.Content[0].Type != "text" {
			t.Fatalf("%s: bad content %s", name, res.Result)
		}
		out := map[string]interface{}{"_isError": r.IsError, "_text": r.Content[0].Text}
		json.Unmarshal([]byte(r.Content[0].Text), &out)
		return out
	}

	r := call("list_collections", `{}`)
	if strings.Contains(r["_text"].(string), "__internal") {
		t.Errorf("internal collection leaked: %s", r["_text"])
	}

	r = call("query_records", `{"collection":"posts","sort":"views","order":"desc","limit":1}`)
	recs := r["records"].([]interface{})
	if len(recs) != 1 || recs[0].(map[string]interface{})["_id"] != "p1" {
		t.Errorf("query sort: %s", r["_text"])
	}

	r = call("query_records", `{"collection":"_users"}`)
	if strings.Contains(r["_text"].(string), "password_hash") {
		t.Errorf("sensitive field leaked")
	}

	r = call("insert_record", `{"collection":"_users","record":{"a":1}}`)
	if r["_isError"] != true {
		t.Errorf("system collection write should fail")
	}

	r = call("update_record", `{"collection":"posts","id":"p2","changes":{"title":"z","views":null}}`)
	if r["_isError"] == true {
		t.Fatalf("update failed: %s", r["_text"])
	}
	if got := p.backend.cols["posts"]["p2"]; got["title"] != "z" || got["views"] != nil {
		t.Errorf("merge update result: %v", got)
	}

	r = call("read_logs", `{"level":"warn"}`)
	if r["count"].(float64) != 2 {
		t.Errorf("read_logs warn: %s", r["_text"])
	}
}

func TestUnknownMethod(t *testing.T) {
	s, _ := newTestServer(t)
	enable(t, s)
	tok := token(t, s)
	rec, res := post(t, s, tok, `{"jsonrpc":"2.0","id":7,"method":"resources/list"}`)
	if rec.Code != 200 || res.Error == nil || res.Error.Code != -32601 {
		t.Fatalf("status %d error %+v", rec.Code, res.Error)
	}
}

func TestBadToken(t *testing.T) {
	s, _ := newTestServer(t)
	enable(t, s)
	for _, tok := range []string{"", "nope", "sbmcp_" + strings.Repeat("x", 43)} {
		rec, res := post(t, s, tok, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
		if rec.Code != http.StatusUnauthorized || res.Error == nil {
			t.Errorf("token %q: status %d", tok, rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("missing WWW-Authenticate")
		}
	}

	// Revoked token stops working.
	tk, plain, _ := s.CreateToken("proj1", "tmp", nil)
	if rec, _ := post(t, s, plain, `{"jsonrpc":"2.0","id":1,"method":"ping"}`); rec.Code != 200 {
		t.Fatalf("fresh token: %d", rec.Code)
	}
	if err := s.DeleteToken("proj1", tk.ID); err != nil {
		t.Fatal(err)
	}
	if rec, _ := post(t, s, plain, `{"jsonrpc":"2.0","id":1,"method":"ping"}`); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token: %d", rec.Code)
	}
}

func TestDisabledProject(t *testing.T) {
	s, _ := newTestServer(t)
	tok := token(t, s) // MCP not enabled (default)
	rec, res := post(t, s, tok, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	if rec.Code != http.StatusForbidden || res.Error == nil {
		t.Fatalf("status %d error %+v", rec.Code, res.Error)
	}
}

func TestGetNotAllowed(t *testing.T) {
	s, _ := newTestServer(t)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/mcp", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("GET status %d", rec.Code)
	}
}

func TestInvalidScope(t *testing.T) {
	s, _ := newTestServer(t)
	if _, _, err := s.CreateToken("proj1", "x", []string{"read", "bogus"}); err == nil {
		t.Fatal("invalid scope should fail")
	}
	if _, _, err := s.CreateToken("proj1", " ", nil); err == nil {
		t.Fatal("empty name should fail")
	}
}

func TestStoreReload(t *testing.T) {
	s, p := newTestServer(t)
	enable(t, s, "read_logs")
	tk, plain, err := s.CreateToken("proj1", "ci", []string{"write", "read"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(plain, TokenPrefix) || tk.Prefix != plain[:12] || tk.Hash == plain {
		t.Fatalf("token shape: %+v", tk)
	}
	// A fresh server over the same DB rebuilds config and token index.
	s2 := NewServer(p)
	rec, _ := post(t, s2, plain, `{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	if rec.Code != 200 {
		t.Fatalf("reloaded token: %d", rec.Code)
	}
	st, _ := s2.Store("proj1")
	cfg := st.Config()
	if !cfg.Enabled || len(cfg.DisabledTools) != 1 {
		t.Errorf("config not persisted: %+v", cfg)
	}
	toks := st.ListTokens()
	if len(toks) != 1 || toks[0].Scopes[0] != "read" || toks[0].LastUsedAt == nil {
		t.Errorf("tokens: %+v", toks)
	}
}
