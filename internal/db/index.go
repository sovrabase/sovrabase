package db

import (
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/pebble"
)

// IndexType defines the behaviour of a secondary index.
type IndexType string

const (
	IndexSimple IndexType = "simple"
	IndexUnique IndexType = "unique"
)

// IndexConfig describes a secondary index on a collection field.
type IndexConfig struct {
	Field string    `json:"field"`
	Type  IndexType `json:"type"` // "simple" or "unique"
}

// Constants for index key prefixes.
const (
	idxMetaPrefix = "__idx_meta__:"   // stores index configs per collection
	idxEntryPrefx = "__idx__:"        // stores index entries
)

// idxMetaKey returns the Pebble key for collection index metadata.
func idxMetaKey(collection string) []byte {
	return []byte(idxMetaPrefix + collection)
}

// idxEntryKey returns the Pebble key for a single index entry.
// Format: __idx__:{collection}:{field}:{value}:{docid}
func idxEntryKey(collection, field, value, docID string) []byte {
	// Use colon as delimiter. Value is included raw — for complex values
	// the callers handle empty/missing fields before calling this.
	return []byte(fmt.Sprintf("%s%s:%s:%s:%s", idxEntryPrefx, collection, field, value, docID))
}

// idxFieldPrefix returns the Pebble key prefix for all entries of an index field.
func idxFieldPrefix(collection, field string) []byte {
	return []byte(fmt.Sprintf("%s%s:%s:", idxEntryPrefx, collection, field))
}

// idxFieldValuePrefix returns the Pebble key prefix for all entries of an index field
// with a specific value.
func idxFieldValuePrefix(collection, field, value string) []byte {
	return []byte(fmt.Sprintf("%s%s:%s:%s:", idxEntryPrefx, collection, field, value))
}

// stripDocIDFromIdxKey extracts the doc ID from the end of an index entry key.
func docIDFromIdxKey(key []byte) string {
	// key format: __idx__:{collection}:{field}:{value}:{docid}
	// Find the last colon.
	s := string(key)
	lastColon := -1
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon == -1 {
		return ""
	}
	return s[lastColon+1:]
}

// fieldValueFromIdxKey extracts the field value from an index entry key.
// The value sits between the 3rd and 4th colon-delimited segments.
// key format: __idx__:{collection}:{field}:{value}:{docid}
func fieldValueFromIdxKey(key []byte) string {
	s := string(key)
	// Skip prefix "__idx__:"
	rest := s[len(idxEntryPrefx):]
	// Split on ':', we need the 3rd colon-delimited part (index 2)
	// Parts: collection, field, value, docid
	colons := 0
	start := 0
	for i := 0; i < len(rest); i++ {
		if rest[i] == ':' {
			colons++
			if colons == 2 {
				start = i + 1
			} else if colons == 3 {
				return rest[start:i]
			}
		}
	}
	return ""
}

// ─── Index CRUD on Engine ─────────────────────────────────────────────────────

// CreateIndex adds a secondary index to a collection.
// For unique indexes, existing documents are checked for conflicts.
func (e *Engine) CreateIndex(collection, field string, idxType IndexType) error {
	// Load existing indexes.
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return fmt.Errorf("db: list indexes: %w", err)
	}

	// Check for duplicate.
	for _, idx := range idxs {
		if idx.Field == field {
			return fmt.Errorf("db: index on field %q already exists in collection %q", field, collection)
		}
	}

	// For unique indexes, validate existing documents have no duplicates.
	if idxType == IndexUnique {
		seen := make(map[string]string) // value -> docID
		docs, scanErr := e.scanCollection(collection)
		if scanErr != nil {
			return fmt.Errorf("db: scan collection for unique check: %w", scanErr)
		}
		for _, doc := range docs {
			val, ok := doc[field]
			if !ok {
				continue
			}
			valStr := fmt.Sprintf("%v", val)
			docID, _ := doc["_id"].(string)
			if existingID, exists := seen[valStr]; exists {
				return fmt.Errorf("db: unique index conflict: field %q value %q exists in docs %s and %s",
					field, valStr, existingID, docID)
			}
			seen[valStr] = docID
		}
	}

	// Add the index config.
	meta := IndexConfig{Field: field, Type: idxType}
	idxs = append(idxs, meta)
	if err := e.saveIndexMeta(collection, idxs); err != nil {
		return err
	}

	// Backfill index entries for existing documents.
	docs, scanErr := e.scanCollection(collection)
	if scanErr != nil {
		return fmt.Errorf("db: scan for backfill: %w", scanErr)
	}
	for _, doc := range docs {
		val, ok := doc[field]
		if !ok {
			continue
		}
		valStr := fmt.Sprintf("%v", val)
		docID, _ := doc["_id"].(string)
		if docID == "" {
			continue
		}
		k := idxEntryKey(collection, field, valStr, docID)
		if err := e.db.Set(k, nil, pebble.Sync); err != nil {
			return fmt.Errorf("db: backfill index entry: %w", err)
		}
	}

	return nil
}

// DropIndex removes a secondary index from a collection and cleans up all
// index entries.
func (e *Engine) DropIndex(collection, field string) error {
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return fmt.Errorf("db: list indexes: %w", err)
	}

	// Remove the config entry.
	var updated []IndexConfig
	found := false
	for _, idx := range idxs {
		if idx.Field == field {
			found = true
		} else {
			updated = append(updated, idx)
		}
	}
	if !found {
		return fmt.Errorf("db: no index on field %q in collection %q", field, collection)
	}

	if err := e.saveIndexMeta(collection, updated); err != nil {
		return fmt.Errorf("db: save index meta: %w", err)
	}

	// Delete all index entries for this field.
	prefix := idxFieldPrefix(collection, field)
	if err := e.deleteByPrefix(prefix); err != nil {
		return fmt.Errorf("db: delete index entries: %w", err)
	}

	return nil
}

// ListIndexes returns all index configurations for a collection.
func (e *Engine) ListIndexes(collection string) ([]IndexConfig, error) {
	key := idxMetaKey(collection)
	val, closer, err := e.db.Get(key)
	if err == pebble.ErrNotFound {
		return []IndexConfig{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("db: get index meta: %w", err)
	}
	defer closer.Close()

	var idxs []IndexConfig
	if err := json.Unmarshal(val, &idxs); err != nil {
		return nil, fmt.Errorf("db: unmarshal index meta: %w", err)
	}
	if idxs == nil {
		return []IndexConfig{}, nil
	}
	return idxs, nil
}

// GetIndex returns the config for a specific index field, or nil if not found.
func (e *Engine) GetIndex(collection, field string) *IndexConfig {
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return nil
	}
	for _, idx := range idxs {
		if idx.Field == field {
			return &idx
		}
	}
	return nil
}

// saveIndexMeta persists the full list of indexes for a collection.
func (e *Engine) saveIndexMeta(collection string, idxs []IndexConfig) error {
	if idxs == nil {
		idxs = []IndexConfig{}
	}
	data, err := json.Marshal(idxs)
	if err != nil {
		return fmt.Errorf("db: marshal index meta: %w", err)
	}
	return e.db.Set(idxMetaKey(collection), data, pebble.Sync)
}

// ─── Index Maintenance (called from Insert/Update/Delete) ─────────────────────

// insertIndexEntries creates index entries for all indexed fields of a document.
// Must be called AFTER the document write is committed.
func (e *Engine) insertIndexEntries(collection string, doc map[string]interface{}) error {
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return err
	}
	if len(idxs) == 0 {
		return nil
	}

	docID, _ := doc["_id"].(string)
	if docID == "" {
		return nil
	}

	for _, idx := range idxs {
		val, ok := doc[idx.Field]
		if !ok {
			continue
		}
		valStr := fmt.Sprintf("%v", val)

		// Unique: check no other doc has this value.
		if idx.Type == IndexUnique {
			existing := e.scanIndexValue(collection, idx.Field, valStr)
			if existing != "" && existing != docID {
				return fmt.Errorf("db: unique constraint violation: field %q value %q already used by doc %s",
					idx.Field, valStr, existing)
			}
		}

		k := idxEntryKey(collection, idx.Field, valStr, docID)
		if err := e.db.Set(k, nil, pebble.Sync); err != nil {
			return fmt.Errorf("db: set index entry: %w", err)
		}
	}
	return nil
}

// deleteIndexEntries removes all index entries for a document.
// Must be called BEFORE the document is deleted, or after reading the old doc.
func (e *Engine) deleteIndexEntries(collection string, doc map[string]interface{}) error {
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return err
	}
	if len(idxs) == 0 {
		return nil
	}

	docID, _ := doc["_id"].(string)
	if docID == "" {
		return nil
	}

	for _, idx := range idxs {
		val, ok := doc[idx.Field]
		if !ok {
			continue
		}
		valStr := fmt.Sprintf("%v", val)
		k := idxEntryKey(collection, idx.Field, valStr, docID)
		if err := e.db.Delete(k, pebble.Sync); err != nil {
			return fmt.Errorf("db: delete index entry: %w", err)
		}
	}
	return nil
}

// updateIndexEntries updates index entries when a document changes.
// It deletes old entries and inserts new ones for fields whose values changed.
func (e *Engine) updateIndexEntries(collection string, oldDoc, newDoc map[string]interface{}) error {
	idxs, err := e.ListIndexes(collection)
	if err != nil {
		return err
	}
	if len(idxs) == 0 {
		return nil
	}

	docID, _ := newDoc["_id"].(string)
	if docID == "" {
		return nil
	}

	for _, idx := range idxs {
		oldVal, oldOK := oldDoc[idx.Field]
		newVal, newOK := newDoc[idx.Field]
		oldStr := fmt.Sprintf("%v", oldVal)
		newStr := fmt.Sprintf("%v", newVal)

		// If the value didn't change, skip.
		if oldOK && newOK && oldStr == newStr {
			continue
		}

		// Delete old entry.
		if oldOK {
			k := idxEntryKey(collection, idx.Field, oldStr, docID)
			if err := e.db.Delete(k, pebble.Sync); err != nil {
				return fmt.Errorf("db: delete old index entry: %w", err)
			}
		}

		// Insert new entry with uniqueness check.
		if newOK {
			if idx.Type == IndexUnique {
				existing := e.scanIndexValue(collection, idx.Field, newStr)
				if existing != "" && existing != docID {
					return fmt.Errorf("db: unique constraint violation: field %q value %q already used by doc %s",
						idx.Field, newStr, existing)
				}
			}
			k := idxEntryKey(collection, idx.Field, newStr, docID)
			if err := e.db.Set(k, nil, pebble.Sync); err != nil {
				return fmt.Errorf("db: set new index entry: %w", err)
			}
		}
	}
	return nil
}

// ─── Index Lookup ─────────────────────────────────────────────────────────────

// scanIndexValue checks if a specific indexed value exists and returns the doc ID.
// Returns "" if not found.
func (e *Engine) scanIndexValue(collection, field, value string) string {
	prefix := idxFieldValuePrefix(collection, field, value)
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return ""
	}
	defer iter.Close()

	if iter.First(); iter.Valid() {
		return docIDFromIdxKey(iter.Key())
	}
	return ""
}

// scanDocsByIndex returns documents from a collection matching specific indexed
// field values. For each matching index entry, it does a point read of the doc.
func (e *Engine) scanDocsByIndex(collection string, indexes []indexMatch, limit, offset int) ([]map[string]interface{}, error) {
	l, o := normalizePagination(limit, offset)

	// We only support the first index match for iteration — we'll post-filter
	// any additional conditions.
	match := indexes[0]
	prefix := idxFieldValuePrefix(collection, match.Field, match.Value)

	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("db: index scan %q: %w", collection, err)
	}
	defer iter.Close()

	var docs []map[string]interface{}
	skipped := 0
	for iter.First(); iter.Valid(); iter.Next() {
		if len(docs) >= l {
			break
		}

		docID := docIDFromIdxKey(iter.Key())
		if docID == "" {
			continue
		}

		doc, err := e.Get(collection, docID)
		if err != nil || doc == nil {
			continue
		}

		// Post-filter: apply remaining conditions.
		if len(indexes) > 1 {
			for _, other := range indexes[1:] {
				got, ok := doc[other.Field]
				if !ok || fmt.Sprintf("%v", got) != other.Value {
					doc = nil
					break
				}
			}
		}
		if doc == nil {
			continue
		}

		if skipped < o {
			skipped++
			continue
		}

		docs = append(docs, doc)
	}
	if err := iter.Error(); err != nil {
		return nil, fmt.Errorf("db: index iter %q: %w", collection, err)
	}
	return docs, nil
}

// indexMatch holds a single field=value condition that can be satisfied
// by an index scan.
type indexMatch struct {
	Field string
	Value string
}

// deleteByPrefix removes all keys sharing a given prefix.
func (e *Engine) deleteByPrefix(prefix []byte) error {
	iter, err := e.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: prefixUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("db: delete by prefix: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		if err := e.db.Delete(iter.Key(), pebble.Sync); err != nil {
			return fmt.Errorf("db: delete key: %w", err)
		}
	}
	return iter.Error()
}
