package replication

import (
	"github.com/ketsuna-org/sovrabase/internal/db"
)

// ReplicatedDB wraps a *db.Engine and a *Node to implement api.DatabaseService.
// Mutations are routed through Node.Write so they go through the WAL/replication
// pipeline. Read operations delegate directly to the underlying engine.
type ReplicatedDB struct {
	engine *db.Engine
	node   *Node
}

// NewReplicatedDB creates a new ReplicatedDB that satisfies the
// api.DatabaseService interface.
func NewReplicatedDB(engine *db.Engine, node *Node) *ReplicatedDB {
	return &ReplicatedDB{
		engine: engine,
		node:   node,
	}
}

// ─── Mutations (through Node.Write) ────────────────────────────────────────

// Insert stores a document. The write goes through the replication node so it
// is recorded in the WAL and replicated to followers.
func (r *ReplicatedDB) Insert(collection, id string, doc map[string]interface{}) error {
	_, err := r.node.Write(collection, OpInsert, id, doc)
	return err
}

// Update replaces an existing document via the replication pipeline.
func (r *ReplicatedDB) Update(collection, id string, doc map[string]interface{}) error {
	_, err := r.node.Write(collection, OpUpdate, id, doc)
	return err
}

// Delete removes a document via the replication pipeline.
func (r *ReplicatedDB) Delete(collection, id string) error {
	_, err := r.node.Write(collection, OpDelete, id, nil)
	return err
}

// CreateCollection registers a new collection via the replication pipeline.
func (r *ReplicatedDB) CreateCollection(name string) error {
	_, err := r.node.Write(name, OpCreateCollection, "", nil)
	return err
}

// DropCollection removes a collection via the replication pipeline.
func (r *ReplicatedDB) DropCollection(name string) error {
	_, err := r.node.Write(name, OpDropCollection, "", nil)
	return err
}

// ─── Reads (direct engine access) ──────────────────────────────────────────

// Get retrieves a document directly from the local engine.
func (r *ReplicatedDB) Get(collection, id string) (map[string]interface{}, error) {
	return r.engine.Get(collection, id)
}

// List returns all documents in a collection from the local engine.
func (r *ReplicatedDB) List(collection string) ([]map[string]interface{}, error) {
	return r.engine.List(collection)
}

// ListPaged returns a paginated slice of documents from the local engine.
func (r *ReplicatedDB) ListPaged(collection string, limit, offset int) ([]map[string]interface{}, error) {
	return r.engine.ListPaged(collection, limit, offset)
}

// Query returns documents matching a filter from the local engine.
func (r *ReplicatedDB) Query(collection string, filter map[string]interface{}, projection []string) ([]map[string]interface{}, error) {
	return r.engine.Query(collection, filter, projection)
}

// QueryPaged returns paginated documents matching a filter from the local engine.
func (r *ReplicatedDB) QueryPaged(collection string, filter map[string]interface{}, projection []string, limit, offset int) ([]map[string]interface{}, error) {
	return r.engine.QueryPaged(collection, filter, projection, limit, offset)
}

// Count returns the total number of documents in a collection from the local engine.
func (r *ReplicatedDB) Count(collection string) (int64, error) {
	return r.engine.Count(collection)
}

// Search performs full-text search on the local engine.
func (r *ReplicatedDB) Search(collection string, query string, fields []string, limit int) ([]map[string]interface{}, error) {
	return r.engine.Search(collection, query, fields, limit)
}

// ─── Index Management (delegate to engine, not yet replicated) ─────────────

// CreateIndex adds a secondary index directly on the engine.
func (r *ReplicatedDB) CreateIndex(collection, field string, idxType db.IndexType) error {
	return r.engine.CreateIndex(collection, field, idxType)
}

// DropIndex removes a secondary index directly on the engine.
func (r *ReplicatedDB) DropIndex(collection, field string) error {
	return r.engine.DropIndex(collection, field)
}

// ListIndexes returns all index configurations for a collection from the engine.
func (r *ReplicatedDB) ListIndexes(collection string) ([]db.IndexConfig, error) {
	return r.engine.ListIndexes(collection)
}

// ─── RLS Rules (delegate to engine, not yet replicated) ────────────────────

// GetRules retrieves the RLS configuration from the engine.
func (r *ReplicatedDB) GetRules(collection string) (*db.RulesConfig, error) {
	return r.engine.GetRules(collection)
}

// SetRules stores the RLS configuration on the engine.
func (r *ReplicatedDB) SetRules(collection string, cfg *db.RulesConfig) error {
	return r.engine.SetRules(collection, cfg)
}
