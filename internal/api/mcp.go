package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/cockroachdb/pebble"

	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/functions"
	"github.com/ketsuna-org/sovrabase/internal/mcp"
	"github.com/ketsuna-org/sovrabase/internal/storage"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// The public API server and the admin server share one MCP server per
// ProjectManager so tokens and the call log are consistent between them.
var (
	mcpServersMu sync.Mutex
	mcpServers   = make(map[*tenant.ProjectManager]*mcp.Server)
)

func mcpServerFor(pm *tenant.ProjectManager, dataDir string) *mcp.Server {
	mcpServersMu.Lock()
	defer mcpServersMu.Unlock()
	if s, ok := mcpServers[pm]; ok {
		return s
	}
	s := mcp.NewServer(&mcpProvider{pm: pm, dataDir: dataDir})
	mcpServers[pm] = s
	return s
}

// mcpProvider adapts the tenant ProjectManager to mcp.Provider.
type mcpProvider struct {
	pm      *tenant.ProjectManager
	dataDir string
}

func (p *mcpProvider) ProjectIDs() []string {
	projects := p.pm.ListProjects()
	ids := make([]string, 0, len(projects))
	for _, proj := range projects {
		if proj.Status == "deleted" {
			continue
		}
		ids = append(ids, proj.ID)
	}
	return ids
}

func (p *mcpProvider) ProjectDB(projectID string) (*pebble.DB, error) {
	env, err := p.pm.GetProjectEnv(projectID)
	if err != nil {
		return nil, err
	}
	return env.Engine.DB(), nil
}

func (p *mcpProvider) Backend(projectID string) (mcp.Backend, error) {
	env, err := p.pm.GetProjectEnv(projectID)
	if err != nil {
		return nil, err
	}
	return &mcpBackend{projectID: projectID, env: env, pm: p.pm, dataDir: p.dataDir}, nil
}

// mcpBackend implements mcp.Backend with the same engine, storage driver,
// function store and request log the admin handlers use.
type mcpBackend struct {
	projectID string
	env       *tenant.ProjectEnv
	pm        *tenant.ProjectManager
	dataDir   string
}

// data returns the replication-aware DB for record operations.
func (b *mcpBackend) data() DatabaseService {
	if b.env.ReplDB != nil {
		return b.env.ReplDB
	}
	return b.env.Engine
}

func (b *mcpBackend) ListCollections() ([]string, error) { return b.env.Engine.ListCollections() }

func (b *mcpBackend) CountRecords(collection string) (int64, error) {
	return b.env.Engine.Count(collection)
}

func (b *mcpBackend) CreateCollection(name string) error { return b.env.Engine.CreateCollection(name) }

func (b *mcpBackend) QueryRecords(collection string, filter map[string]interface{}, limit, offset int) ([]map[string]interface{}, error) {
	if limit > 0 {
		if len(filter) == 0 {
			return b.data().ListPaged(collection, limit, offset)
		}
		return b.data().QueryPaged(collection, filter, nil, limit, offset)
	}
	if len(filter) == 0 {
		return b.data().List(collection)
	}
	return b.data().Query(collection, filter, nil)
}

func (b *mcpBackend) GetRecord(collection, id string) (map[string]interface{}, error) {
	return b.data().Get(collection, id)
}

func (b *mcpBackend) InsertRecord(collection, id string, doc map[string]interface{}) error {
	return b.data().Insert(collection, id, doc)
}

func (b *mcpBackend) UpdateRecord(collection, id string, doc map[string]interface{}) error {
	return b.data().Update(collection, id, doc)
}

func (b *mcpBackend) DeleteRecord(collection, id string) error {
	return b.data().Delete(collection, id)
}

func (b *mcpBackend) ListIndexes(collection string) ([]mcp.IndexInfo, error) {
	idxs, err := b.env.Engine.ListIndexes(collection)
	if err != nil {
		return nil, err
	}
	out := make([]mcp.IndexInfo, 0, len(idxs))
	for _, i := range idxs {
		out = append(out, mcp.IndexInfo{Field: i.Field, Type: string(i.Type)})
	}
	return out, nil
}

func (b *mcpBackend) CreateIndex(collection, field string, unique bool) error {
	t := db.IndexSimple
	if unique {
		t = db.IndexUnique
	}
	return b.env.Engine.CreateIndex(collection, field, t)
}

func (b *mcpBackend) GetRules(collection string) (interface{}, error) {
	return b.env.Engine.GetRules(collection)
}

func (b *mcpBackend) ListFunctions() ([]mcp.FunctionInfo, error) {
	store, err := getFunctionStore(b.projectID, b.pm)
	if err != nil {
		return nil, err
	}
	fns, err := store.List()
	if err != nil {
		return nil, err
	}
	out := make([]mcp.FunctionInfo, 0, len(fns))
	for _, fn := range fns {
		out = append(out, mcp.FunctionInfo{
			Name:       fn.Name,
			Language:   fn.Language,
			Trigger:    string(fn.Trigger),
			Collection: fn.Collection,
			Schedule:   fn.Schedule,
			Public:     fn.Public,
			Enabled:    fn.Enabled,
		})
	}
	return out, nil
}

func (b *mcpBackend) InvokeFunction(ctx context.Context, name string, payload map[string]interface{}) (interface{}, error) {
	store, err := getFunctionStore(b.projectID, b.pm)
	if err != nil {
		return nil, err
	}
	fn, err := store.GetByName(name)
	if err != nil {
		return nil, fmt.Errorf("function %q not found", name)
	}
	if !fn.Enabled {
		return nil, fmt.Errorf("function %q is disabled", name)
	}
	ctx, cancel := context.WithTimeout(ctx, timeLimit(fn))
	defer cancel()

	exec := FunctionExecutor()
	if fn.Trigger == functions.TriggerHTTP {
		var body interface{}
		if payload != nil {
			body = payload
		}
		result, _, err := exec.InvokeHTTP(ctx, b.projectID, fn.Name, functions.HTTPRequest{
			Method:  "POST",
			Path:    "/api/v1/functions/" + fn.Name,
			Query:   map[string]string{},
			Headers: map[string]string{"X-Invoked-By": "mcp"},
			Body:    body,
		})
		if err != nil {
			return nil, err
		}
		return testRunPayload(result), nil
	}
	event := functions.EventPayload{Type: string(fn.Trigger), Collection: fn.Collection, Data: payload}
	return testRunPayload(exec.InvokeEvent(ctx, b.projectID, fn, event)), nil
}

func (b *mcpBackend) ListBuckets(ctx context.Context) ([]string, error) {
	return b.env.Storage.ListBuckets(ctx)
}

func (b *mcpBackend) ListFiles(ctx context.Context, bucket, prefix string) (interface{}, error) {
	files, err := b.env.Storage.List(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}
	if files == nil {
		files = []storage.FileInfo{}
	}
	return files, nil
}

// ReadLogs tails the project's requests.log (the file behind
// GET /admin/projects/{id}/logs), newest first.
func (b *mcpBackend) ReadLogs(max int) ([]map[string]interface{}, error) {
	logFile := filepath.Join(b.dataDir, "projects", b.projectID, "requests.log")
	f, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []map[string]interface{}{}, nil
		}
		return nil, err
	}
	defer f.Close()

	ring := make([]string, max)
	n := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		ring[n%max] = scanner.Text()
		n++
	}
	count := n
	if count > max {
		count = max
	}
	out := make([]map[string]interface{}, 0, count)
	for i := 0; i < count; i++ {
		line := ring[(n-1-i+max)%max]
		var entry map[string]interface{}
		if json.Unmarshal([]byte(line), &entry) == nil {
			out = append(out, entry)
		}
	}
	return out, nil
}
