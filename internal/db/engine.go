package db

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
)

const (
	// metaPrefix is the key prefix for collection metadata keys.
	metaPrefix = "__meta__:"
)

// Engine is a JSON document store backed by Pebble (LSM-based KV store).
type Engine struct {
	db     *pebble.DB
	dir    string // temp directory for cleanup, empty for user-provided paths
	closed bool
	mu     sync.Mutex
}

// NewEngine opens (or creates) a Pebble database at the given directory.
func NewEngine(dataDir string) (*Engine, error) {
	opts := &pebble.Options{}
	db, err := pebble.Open(dataDir, opts)
	if err != nil {
		return nil, fmt.Errorf("db: open pebble: %w", err)
	}
	return &Engine{db: db}, nil
}

// NewMemEngine creates an Engine in a temporary directory, intended for tests.
func NewMemEngine() (*Engine, error) {
	dir, err := os.MkdirTemp("", "sovrabase-mem-*")
	if err != nil {
		return nil, fmt.Errorf("db: create temp dir: %w", err)
	}
	db, err := pebble.Open(dir, &pebble.Options{})
	if err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("db: open mem pebble: %w", err)
	}
	return &Engine{db: db, dir: dir}, nil
}

// Close gracefully shuts down the database. Safe to call multiple times.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	err := e.db.Close()
	if e.dir != "" {
		os.RemoveAll(e.dir)
	}
	return err
}

// collectionMetaKey returns the Pebble key for collection metadata.
func collectionMetaKey(name string) []byte {
	return []byte(metaPrefix + name)
}

// docKey returns the Pebble key for a document.
func docKey(collection, id string) []byte {
	return []byte(collection + ":" + id)
}

// CreateCollection registers a new collection namespace.
func (e *Engine) CreateCollection(name string) error {
	metaKey := collectionMetaKey(name)

	_, closer, err := e.db.Get(metaKey)
	if err != nil && err != pebble.ErrNotFound {
		return fmt.Errorf("db: check collection %q: %w", name, err)
	}
	if closer != nil {
		closer.Close()
		return fmt.Errorf("db: collection %q already exists", name)
	}

	meta := map[string]interface{}{
		"name": name,
	}
	metaBytes, err := MarshalMap(meta)
	if err != nil {
		return fmt.Errorf("db: marshal collection meta: %w", err)
	}

	if err := e.db.Set(metaKey, metaBytes, pebble.Sync); err != nil {
		return fmt.Errorf("db: create collection %q: %w", name, err)
	}
	return nil
}

// ListCollections returns the names of all collections.
func (e *Engine) ListCollections() ([]string, error) {
	prefix := []byte(metaPrefix)
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("db: list collections iter: %w", err)
	}
	defer iter.Close()

	var collections []string
	for iter.First(); iter.Valid(); iter.Next() {
		keyStr := string(iter.Key())
		name := strings.TrimPrefix(keyStr, metaPrefix)
		collections = append(collections, name)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: list collections: %w", err)
	}
	return collections, nil
}

// DropCollection removes a collection and all its documents.
func (e *Engine) DropCollection(name string) error {
	metaKey := collectionMetaKey(name)

	// Check the collection exists.
	_, closer, err := e.db.Get(metaKey)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("db: collection %q not found", name)
	}
	if err != nil {
		return fmt.Errorf("db: check collection %q: %w", name, err)
	}
	closer.Close()

	// Delete all documents belonging to this collection.
	prefix := []byte(name + ":")
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("db: scan collection %q: %w", name, err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if err := e.db.Delete(iter.Key(), pebble.Sync); err != nil {
			return fmt.Errorf("db: delete doc from %q: %w", name, err)
		}
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("db: iterate collection %q: %w", name, err)
	}

	// Remove the collection metadata.
	if err := e.db.Delete(metaKey, pebble.Sync); err != nil {
		return fmt.Errorf("db: delete collection meta %q: %w", name, err)
	}
	return nil
}

// Insert stores a JSON document under the given collection and id.
// If id is empty, a UUIDv4 is generated and returned in the document under "_id".
// The stored document will include the key "_id".
func (e *Engine) Insert(collection, id string, doc map[string]interface{}) error {
	metaKey := collectionMetaKey(collection)
	_, closer, err := e.db.Get(metaKey)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("db: collection %q not found", collection)
	}
	if err != nil {
		return fmt.Errorf("db: check collection %q: %w", collection, err)
	}
	closer.Close()

	if id == "" {
		id = uuid.New().String()
	}
	doc["_id"] = id

	key := docKey(collection, id)
	data, err := MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("db: marshal doc: %w", err)
	}

	if err := e.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("db: insert doc: %w", err)
	}
	return nil
}

// Get retrieves a document by collection and id.
// Returns nil,nil if the document is not found.
func (e *Engine) Get(collection, id string) (map[string]interface{}, error) {
	key := docKey(collection, id)
	val, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get doc: %w", err)
	}
	defer closer.Close()

	return UnmarshalMap(val)
}

// Update replaces an existing document. Returns an error if the document does not exist.
func (e *Engine) Update(collection, id string, doc map[string]interface{}) error {
	key := docKey(collection, id)

	_, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("db: doc %q not found in collection %q", id, collection)
	}
	if err != nil {
		return fmt.Errorf("db: check doc: %w", err)
	}
	closer.Close()

	doc["_id"] = id
	data, err := MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("db: marshal doc: %w", err)
	}

	if err := e.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("db: update doc: %w", err)
	}
	return nil
}

// Delete removes a document by collection and id. Returns an error if not found.
func (e *Engine) Delete(collection, id string) error {
	key := docKey(collection, id)

	_, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return fmt.Errorf("db: doc %q not found in collection %q", id, collection)
	}
	if err != nil {
		return fmt.Errorf("db: check doc: %w", err)
	}
	closer.Close()

	if err := e.db.Delete(key, pebble.Sync); err != nil {
		return fmt.Errorf("db: delete doc: %w", err)
	}
	return nil
}

// List returns all documents in a collection.
func (e *Engine) List(collection string) ([]map[string]interface{}, error) {
	return e.scanCollection(collection)
}

// Query returns documents in a collection matching the given filter.
// Only top-level equality filters are supported (e.g., {"status": "active"}).
func (e *Engine) Query(collection string, filter map[string]interface{}) ([]map[string]interface{}, error) {
	docs, err := e.scanCollection(collection)
	if err != nil {
		return nil, err
	}
	if len(filter) == 0 {
		return docs, nil
	}

	var result []map[string]interface{}
	for _, doc := range docs {
		if matchFilter(doc, filter) {
			result = append(result, doc)
		}
	}
	return result, nil
}

// scanCollection iterates over all documents in a collection.
func (e *Engine) scanCollection(collection string) ([]map[string]interface{}, error) {
	prefix := []byte(collection + ":")
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("db: scan collection %q: %w", collection, err)
	}
	defer iter.Close()

	var docs []map[string]interface{}
	for iter.First(); iter.Valid(); iter.Next() {
		doc, err := UnmarshalMap(iter.Value())
		if err != nil {
			return nil, fmt.Errorf("db: unmarshal doc: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: iterate collection %q: %w", collection, err)
	}
	return docs, nil
}

// matchFilter returns true if doc matches all filter conditions.
// Only top-level equality checks are performed.
func matchFilter(doc map[string]interface{}, filter map[string]interface{}) bool {
	for k, want := range filter {
		got, ok := doc[k]
		if !ok {
			return false
		}
		// Simple equality check (not deep).
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return false
		}
	}
	return true
}

// prefixUpperBound returns the key that sits just beyond all keys sharing a
// given prefix. For byte-ordered iteration this is typically prefix + \xFF\xFF…
// For Pebble we compute prefixUpperBound by incrementing the last byte.
func prefixUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	// Append the maximum byte; this is effectively prefix + \xff which is the
	// exclusive upper bound for prefix scans in Pebble.
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	// Prefix is all 0xff; append 0x00 to make it longer.
	return append(prefix, 0x00)
}
