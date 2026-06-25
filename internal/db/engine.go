package db

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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

// DB returns the underlying Pebble database instance.
// This is exposed for backup/checkpoint operations.
func (e *Engine) DB() *pebble.DB { return e.db }

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
	if err := e.deleteByPrefix(prefix); err != nil {
		return fmt.Errorf("db: delete docs from %q: %w", name, err)
	}

	// Delete index entries for this collection.
	idxPrefix := []byte(fmt.Sprintf("%s%s:", idxEntryPrefx, name))
	_ = e.deleteByPrefix(idxPrefix)

	// Delete index metadata.
	_ = e.db.Delete(idxMetaKey(name), pebble.Sync)

	// Delete RLS rules for this collection.
	_ = e.db.Delete([]byte("__rules__:"+name), pebble.Sync)

	// Remove the collection metadata.
	if err := e.db.Delete(metaKey, pebble.Sync); err != nil {
		return fmt.Errorf("db: delete collection meta %q: %w", name, err)
	}
	return nil
}

// Insert stores a JSON document under the given collection and id.
// If id is empty, a UUIDv4 is generated and returned in the document under "_id".
// The stored document will include the keys "_id", "_createdAt", "_updatedAt".
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
	now := time.Now().UTC()
	doc["_id"] = id
	doc["_createdAt"] = now
	doc["_updatedAt"] = now

	key := docKey(collection, id)
	data, err := MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("db: marshal doc: %w", err)
	}

	if err := e.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("db: insert doc: %w", err)
	}

	// Maintain secondary indexes.
	if err := e.insertIndexEntries(collection, doc); err != nil {
		return err
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
// Automatically sets _updatedAt and preserves _createdAt if not provided.
func (e *Engine) Update(collection, id string, doc map[string]interface{}) error {
	key := docKey(collection, id)

	existing, err := e.getDoc(key)
	if err != nil {
		return fmt.Errorf("db: doc %q not found in collection %q", id, collection)
	}

	// Preserve _createdAt from existing doc if not explicitly set.
	if _, has := doc["_createdAt"]; !has {
		if createdAt, ok := existing["_createdAt"]; ok {
			doc["_createdAt"] = createdAt
		}
	}

	doc["_id"] = id
	doc["_updatedAt"] = time.Now().UTC()

	data, err := MarshalMap(doc)
	if err != nil {
		return fmt.Errorf("db: marshal doc: %w", err)
	}

	if err := e.db.Set(key, data, pebble.Sync); err != nil {
		return fmt.Errorf("db: update doc: %w", err)
	}

	// Maintain secondary indexes.
	if err := e.updateIndexEntries(collection, existing, doc); err != nil {
		return err
	}
	return nil
}

// getDoc is an internal helper that reads a document by raw key.
func (e *Engine) getDoc(key []byte) (map[string]interface{}, error) {
	val, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return nil, fmt.Errorf("db: doc not found")
	}
	if err != nil {
		return nil, fmt.Errorf("db: get doc: %w", err)
	}
	defer closer.Close()
	return UnmarshalMap(val)
}

// Delete removes a document by collection and id. Returns an error if not found.
func (e *Engine) Delete(collection, id string) error {
	key := docKey(collection, id)

	// Read the existing document so we can clean up index entries.
	existing, err := e.getDoc(key)
	if err != nil {
		return fmt.Errorf("db: doc %q not found in collection %q", id, collection)
	}

	// Remove index entries before deleting the document.
	if err := e.deleteIndexEntries(collection, existing); err != nil {
		return err
	}

	if err := e.db.Delete(key, pebble.Sync); err != nil {
		return fmt.Errorf("db: delete doc: %w", err)
	}
	return nil
}

// List returns all documents in a collection.
func (e *Engine) List(collection string) ([]map[string]interface{}, error) {
	return e.scanCollection(collection)
}

// ListPaged returns a paginated slice of documents from a collection.
// limit is capped at [1, 1000], defaulting to 50. offset skips that many documents.
func (e *Engine) ListPaged(collection string, limit, offset int) ([]map[string]interface{}, error) {
	return e.scanCollectionPaged(collection, limit, offset)
}

// Count returns the total number of documents in a collection.
func (e *Engine) Count(collection string) (int64, error) {
	prefix := []byte(collection + ":")
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("db: count collection %q: %w", collection, err)
	}
	defer iter.Close()

	var count int64
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	if err := iter.Error(); err != nil {
		return 0, fmt.Errorf("db: count iter %q: %w", collection, err)
	}
	return count, nil
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

// scanCollectionPaged iterates over a range of documents in a collection.
func (e *Engine) scanCollectionPaged(collection string, limit, offset int) ([]map[string]interface{}, error) {
	l, o := normalizePagination(limit, offset)

	prefix := []byte(collection + ":")
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("db: scan paged %q: %w", collection, err)
	}
	defer iter.Close()

	var docs []map[string]interface{}
	skipped := 0
	for iter.First(); iter.Valid(); iter.Next() {
		if skipped < o {
			skipped++
			continue
		}
		if len(docs) >= l {
			break
		}
		doc, err := UnmarshalMap(iter.Value())
		if err != nil {
			return nil, fmt.Errorf("db: unmarshal doc: %w", err)
		}
		docs = append(docs, doc)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: iterate paged %q: %w", collection, err)
	}
	return docs, nil
}

// normalizePagination returns sanitized limit and offset values.
func normalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// RulesConfig holds the RLS configuration for a collection.
type RulesConfig struct {
	Enabled bool              `json:"enabled"`
	Rules   map[string]string `json:"rules"` // action -> expression
}

// GetRules retrieves the RLS configuration for a collection.
func (e *Engine) GetRules(collection string) (*RulesConfig, error) {
	key := []byte("__rules__:" + collection)
	val, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return &RulesConfig{Enabled: false, Rules: map[string]string{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get rules: %w", err)
	}
	defer closer.Close()

	var cfg RulesConfig
	if err := json.Unmarshal(val, &cfg); err != nil {
		return nil, fmt.Errorf("db: unmarshal rules: %w", err)
	}
	return &cfg, nil
}

// SetRules stores the RLS configuration for a collection.
func (e *Engine) SetRules(collection string, cfg *RulesConfig) error {
	key := []byte("__rules__:" + collection)
	val, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("db: marshal rules: %w", err)
	}
	return e.db.Set(key, val, pebble.Sync)
}

// Query returns documents in a collection matching the given filter.
// Supports both simple equality filters and advanced comparisons, as well as projection.
// Note: Query does not use secondary indexes — it does a full scan. Use QueryPaged
// for indexed optimisations.
func (e *Engine) Query(collection string, filter map[string]interface{}, projection []string) ([]map[string]interface{}, error) {
	docs, err := e.scanCollection(collection)
	if err != nil {
		return nil, err
	}

	return applyQuery(docs, filter, projection), nil
}

// QueryPaged returns a paginated slice of documents matching the filter.
// Uses secondary indexes when available for efficient filtering — falls back to
// full collection scan otherwise.
func (e *Engine) QueryPaged(collection string, filter map[string]interface{}, projection []string, limit, offset int) ([]map[string]interface{}, error) {
	// Try to use a secondary index.
	var matches []indexMatch
	if len(filter) > 0 {
		for k, v := range filter {
			// Only simple equality filters can use indexes.
			if _, isMap := v.(map[string]interface{}); isMap {
				continue
			}
			idx := e.GetIndex(collection, k)
			if idx != nil {
				matches = append(matches, indexMatch{Field: k, Value: fmt.Sprintf("%v", v)})
			}
		}
	}

	// If we have at least one index match, use the index scan.
	if len(matches) > 0 {
		docs, err := e.scanDocsByIndex(collection, matches, limit, offset)
		if err != nil {
			return nil, err
		}
		// Apply projections after index lookup.
		if len(projection) > 0 {
			for i, doc := range docs {
				docs[i] = projectDoc(doc, projection)
			}
		}
		return docs, nil
	}

	// Fallback: full collection scan with in-memory filtering.
	l, o := normalizePagination(limit, offset)

	prefix := []byte(collection + ":")
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("db: query paged %q: %w", collection, err)
	}
	defer iter.Close()

	var matched []map[string]interface{}
	skipped := 0
	for iter.First(); iter.Valid(); iter.Next() {
		if len(matched) >= l {
			break
		}

		doc, err := UnmarshalMap(iter.Value())
		if err != nil {
			return nil, fmt.Errorf("db: unmarshal doc: %w", err)
		}

		if len(filter) > 0 && !matchFilter(doc, filter) {
			continue
		}

		if skipped < o {
			skipped++
			continue
		}

		if len(projection) > 0 {
			doc = projectDoc(doc, projection)
		}
		matched = append(matched, doc)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: iterate qpaged %q: %w", collection, err)
	}
	return matched, nil
}

// applyQuery filters and projects documents in memory.
func applyQuery(docs []map[string]interface{}, filter map[string]interface{}, projection []string) []map[string]interface{} {
	var result []map[string]interface{}
	for _, doc := range docs {
		if len(filter) == 0 || matchFilter(doc, filter) {
			if len(projection) > 0 {
				doc = projectDoc(doc, projection)
			}
			result = append(result, doc)
		}
	}
	return result
}

// projectDoc returns a new map with only the specified fields plus _id.
func projectDoc(doc map[string]interface{}, projection []string) map[string]interface{} {
	projected := make(map[string]interface{})
	if idVal, ok := doc["_id"]; ok {
		projected["_id"] = idVal
	}
	for _, field := range projection {
		if val, ok := doc[field]; ok {
			projected[field] = val
		}
	}
	return projected
}

// matchFilter returns true if doc matches all filter conditions.
func matchFilter(doc map[string]interface{}, filter map[string]interface{}) bool {
	for k, want := range filter {
		got, ok := doc[k]

		if opMap, isMap := want.(map[string]interface{}); isMap && len(opMap) > 0 {
			if !ok {
				got = nil
			}
			for op, val := range opMap {
				if !compareOp(got, ok, op, val) {
					return false
				}
			}
			continue
		}

		if !ok {
			return false
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return false
		}
	}
	return true
}

func compareOp(got interface{}, ok bool, op string, want interface{}) bool {
	gotStr := fmt.Sprint(got)
	wantStr := fmt.Sprint(want)

	gotNum, gotIsNum := toFloat64(got)
	wantNum, wantIsNum := toFloat64(want)
	numeric := gotIsNum && wantIsNum

	switch op {
	case "$eq":
		if numeric {
			return gotNum == wantNum
		}
		return gotStr == wantStr
	case "$ne":
		if numeric {
			return gotNum != wantNum
		}
		return gotStr != wantStr
	case "$gt":
		if numeric {
			return gotNum > wantNum
		}
		return gotStr > wantStr
	case "$gte":
		if numeric {
			return gotNum >= wantNum
		}
		return gotStr >= wantStr
	case "$lt":
		if numeric {
			return gotNum < wantNum
		}
		return gotStr < wantStr
	case "$lte":
		if numeric {
			return gotNum <= wantNum
		}
		return gotStr <= wantStr
	case "$contains":
		return strings.Contains(strings.ToLower(gotStr), strings.ToLower(wantStr))
	case "$startsWith":
		return strings.HasPrefix(gotStr, wantStr)
	case "$exists":
		wantBool, okBool := want.(bool)
		if !okBool {
			wantBool = (fmt.Sprint(want) == "true" || fmt.Sprint(want) == "1")
		}
		return ok == wantBool
	case "$regex":
		pattern, okStr := want.(string)
		if !okStr {
			pattern = fmt.Sprint(want)
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(gotStr)
	case "$in":
		arr, okList := want.([]interface{})
		if !okList {
			return false
		}
		for _, item := range arr {
			if itemEquals(got, item) {
				return true
			}
		}
		return false
	case "$nin":
		arr, okList := want.([]interface{})
		if !okList {
			return true
		}
		for _, item := range arr {
			if itemEquals(got, item) {
				return false
			}
		}
		return true
	}
	return false
}

func itemEquals(got interface{}, want interface{}) bool {
	gotStr := fmt.Sprint(got)
	wantStr := fmt.Sprint(want)
	gotNum, gotIsNum := toFloat64(got)
	wantNum, wantIsNum := toFloat64(want)
	if gotIsNum && wantIsNum {
		return gotNum == wantNum
	}
	return gotStr == wantStr
}

func toFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch val := v.(type) {
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	}
	return 0, false
}

// Search performs full-text search across documents in a collection.
// It tokenizes the query into lowercase keywords, scans all documents, and
// scores each document by total keyword occurrences across specified fields.
// Results are sorted by score descending and limited to `limit` (default 50).
// A metadata field "_search_score" is added to each returned document.
// If fields is empty/nil, all string fields in each document are searched.
func (e *Engine) Search(collection string, query string, fields []string, limit int) ([]map[string]interface{}, error) {
	if limit <= 0 {
		limit = 50
	}

	// Tokenize the query into lowercase keywords.
	keywords := tokenizeSearchQuery(query)
	if len(keywords) == 0 {
		return []map[string]interface{}{}, nil
	}

	// Scan all documents in the collection.
	docs, err := e.scanCollection(collection)
	if err != nil {
		return nil, err
	}

	type scoredDoc struct {
		doc   map[string]interface{}
		score int
	}

	var scored []scoredDoc
	for _, doc := range docs {
		// Determine which fields to search.
		var searchFields []string
		if len(fields) > 0 {
			searchFields = fields
		} else {
			// Search all string fields in the document.
			for k, v := range doc {
				if _, ok := v.(string); ok {
					searchFields = append(searchFields, k)
				}
			}
		}

		totalScore := 0
		for _, field := range searchFields {
			raw, ok := doc[field]
			if !ok {
				continue
			}
			strVal, ok := raw.(string)
			if !ok {
				continue
			}
			lowerVal := strings.ToLower(strVal)
			for _, kw := range keywords {
				totalScore += strings.Count(lowerVal, kw)
			}
		}

		if totalScore > 0 {
			scored = append(scored, scoredDoc{doc: doc, score: totalScore})
		}
	}

	// Sort by score descending.
	// Use simple insertion sort for typical result sizes (small).
	for i := 1; i < len(scored); i++ {
		for j := i; j > 0 && scored[j-1].score < scored[j].score; j-- {
			scored[j-1], scored[j] = scored[j], scored[j-1]
		}
	}

	// Limit results.
	if len(scored) > limit {
		scored = scored[:limit]
	}

	// Build result with _search_score.
	result := make([]map[string]interface{}, len(scored))
	for i, s := range scored {
		r := make(map[string]interface{})
		for k, v := range s.doc {
			r[k] = v
		}
		r["_search_score"] = s.score
		result[i] = r
	}

	return result, nil
}

// tokenizeSearchQuery splits a query string into lowercase keywords,
// separating on whitespace and common punctuation.
func tokenizeSearchQuery(query string) []string {
	// Split on non-alphanumeric characters (excluding underscores within words)
	// to capture word boundaries.
	re := regexp.MustCompile(`[^a-zA-Z0-9_]+`)
	tokens := re.Split(query, -1)
	var result []string
	for _, t := range tokens {
		t = strings.TrimSpace(t)
		if t != "" {
			result = append(result, strings.ToLower(t))
		}
	}
	return result
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

// StorageAnalysis represents the detailed breakdown of Pebble database storage.
type StorageAnalysis struct {
	TotalSize    int64                         `json:"total_size"`
	MetadataSize int64                         `json:"metadata_size"`
	IndexSize    int64                         `json:"index_size"`
	Collections  map[string]*CollectionStorage `json:"collections"`
}

// CollectionStorage holds storage statistics for a single collection.
type CollectionStorage struct {
	DocumentCount int64 `json:"document_count"`
	DocumentSize  int64 `json:"document_size"`
	IndexSize     int64 `json:"index_size"`
}

// AnalyzeStorage scans the Pebble database and returns a breakdown of storage usage.
func (e *Engine) AnalyzeStorage() (*StorageAnalysis, error) {
	iter, err := e.db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil, fmt.Errorf("db: analyze storage iter: %w", err)
	}
	defer iter.Close()

	analysis := &StorageAnalysis{
		Collections: make(map[string]*CollectionStorage),
	}

	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		val := iter.Value()
		size := int64(len(key) + len(val))
		analysis.TotalSize += size

		keyStr := string(key)
		if strings.HasPrefix(keyStr, metaPrefix) { // "__meta__:"
			analysis.MetadataSize += size
			collName := strings.TrimPrefix(keyStr, metaPrefix)
			c := getOrCreateCollStorage(analysis.Collections, collName)
			c.DocumentSize += size
		} else if strings.HasPrefix(keyStr, idxMetaPrefix) { // "__idx_meta__:"
			analysis.MetadataSize += size
			collName := strings.TrimPrefix(keyStr, idxMetaPrefix)
			c := getOrCreateCollStorage(analysis.Collections, collName)
			c.IndexSize += size
		} else if strings.HasPrefix(keyStr, idxEntryPrefx) { // "__idx__:"
			analysis.IndexSize += size
			rest := strings.TrimPrefix(keyStr, idxEntryPrefx)
			parts := strings.SplitN(rest, ":", 2)
			if len(parts) > 0 {
				collName := parts[0]
				c := getOrCreateCollStorage(analysis.Collections, collName)
				c.IndexSize += size
			}
		} else {
			// Normal document key: {collection}:{id}
			parts := strings.SplitN(keyStr, ":", 2)
			if len(parts) == 2 {
				collName := parts[0]
				c := getOrCreateCollStorage(analysis.Collections, collName)
				c.DocumentCount++
				c.DocumentSize += size
			} else {
				// Unknown key format, count as metadata
				analysis.MetadataSize += size
			}
		}
	}

	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: analyze storage iterate: %w", err)
	}

	return analysis, nil
}

func getOrCreateCollStorage(m map[string]*CollectionStorage, name string) *CollectionStorage {
	c, ok := m[name]
	if !ok {
		c = &CollectionStorage{}
		m[name] = c
	}
	return c
}

