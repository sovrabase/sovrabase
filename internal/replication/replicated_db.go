package replication

import (
	"github.com/ketsuna-org/sovrabase/internal/db"
)

// ReplicatedDB wraps a *db.Engine and a *ClusterNode for a single namespace.
// Mutations go through the WAL/replication pipeline; reads hit the local engine.
type ReplicatedDB struct {
	engine    *db.Engine
	cluster   *ClusterNode
	namespace string
}

// NewReplicatedDB creates a ReplicatedDB for the global namespace.
func NewReplicatedDB(engine *db.Engine, node *ClusterNode) *ReplicatedDB {
	return NewNamespacedReplicatedDB(engine, node, NamespaceGlobal)
}

// NewNamespacedReplicatedDB creates a ReplicatedDB bound to a specific namespace.
func NewNamespacedReplicatedDB(engine *db.Engine, cluster *ClusterNode, namespace string) *ReplicatedDB {
	return &ReplicatedDB{
		engine:    engine,
		cluster:   cluster,
		namespace: NormalizeNamespace(namespace),
	}
}

// RawEngine returns the underlying Pebble engine (for internal stores, checkpoints).
func (r *ReplicatedDB) RawEngine() *db.Engine {
	return r.engine
}

func (r *ReplicatedDB) write(op Operation, collection, id string, doc map[string]interface{}) error {
	_, err := r.cluster.WriteNS(r.namespace, collection, op, id, doc)
	return err
}

func (r *ReplicatedDB) Insert(collection, id string, doc map[string]interface{}) error {
	return r.write(OpInsert, collection, id, doc)
}

func (r *ReplicatedDB) Update(collection, id string, doc map[string]interface{}) error {
	return r.write(OpUpdate, collection, id, doc)
}

func (r *ReplicatedDB) Delete(collection, id string) error {
	return r.write(OpDelete, collection, id, nil)
}

func (r *ReplicatedDB) CreateCollection(name string) error {
	return r.write(OpCreateCollection, name, "", nil)
}

func (r *ReplicatedDB) DropCollection(name string) error {
	return r.write(OpDropCollection, name, "", nil)
}

func (r *ReplicatedDB) Get(collection, id string) (map[string]interface{}, error) {
	return r.engine.Get(collection, id)
}

func (r *ReplicatedDB) List(collection string) ([]map[string]interface{}, error) {
	return r.engine.List(collection)
}

func (r *ReplicatedDB) ListPaged(collection string, limit, offset int) ([]map[string]interface{}, error) {
	return r.engine.ListPaged(collection, limit, offset)
}

func (r *ReplicatedDB) Query(collection string, filter map[string]interface{}, projection []string) ([]map[string]interface{}, error) {
	return r.engine.Query(collection, filter, projection)
}

func (r *ReplicatedDB) QueryPaged(collection string, filter map[string]interface{}, projection []string, limit, offset int) ([]map[string]interface{}, error) {
	return r.engine.QueryPaged(collection, filter, projection, limit, offset)
}

func (r *ReplicatedDB) Count(collection string) (int64, error) {
	return r.engine.Count(collection)
}

func (r *ReplicatedDB) Search(collection string, query string, fields []string, limit int) ([]map[string]interface{}, error) {
	return r.engine.Search(collection, query, fields, limit)
}

func (r *ReplicatedDB) CreateIndex(collection, field string, idxType db.IndexType) error {
	return r.cluster.WriteCreateIndex(r.namespace, collection, field, idxType)
}

func (r *ReplicatedDB) DropIndex(collection, field string) error {
	return r.cluster.WriteDropIndex(r.namespace, collection, field)
}

func (r *ReplicatedDB) ListIndexes(collection string) ([]db.IndexConfig, error) {
	return r.engine.ListIndexes(collection)
}

func (r *ReplicatedDB) GetRules(collection string) (*db.RulesConfig, error) {
	return r.engine.GetRules(collection)
}

func (r *ReplicatedDB) SetRules(collection string, cfg *db.RulesConfig) error {
	return r.cluster.WriteSetRules(r.namespace, collection, cfg)
}
