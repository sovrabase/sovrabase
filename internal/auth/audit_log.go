package auth

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/google/uuid"
)

// AuditAction represents a type of administrative action.
type AuditAction string

// AuditEntry represents a single audit log entry for admin actions.
type AuditEntry struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	AdminID     string                 `json:"admin_id"`
	AdminEmail  string                 `json:"admin_email"`
	Action      string                 `json:"action"`
	TargetType  string                 `json:"target_type"`
	TargetID    string                 `json:"target_id"`
	Details     map[string]interface{} `json:"details,omitempty"`
	IP          string                 `json:"ip,omitempty"`
	Success     bool                   `json:"success"`
}

// AuditStore manages a Pebble-backed audit log for administrative actions.
type AuditStore struct {
	db *pebble.DB
}

// NewAuditStore creates an AuditStore backed by the given Pebble database.
func NewAuditStore(db *pebble.DB) *AuditStore {
	return &AuditStore{db: db}
}

// key helpers

func auditEntryKey(timestamp time.Time, id string) []byte {
	// Keys are ordered by timestamp descending, so we invert the timestamp
	// by using a max timestamp minus the actual timestamp.
	// This means reverse iteration with SeekLT is not strictly needed,
	// but we format it so that lexicographic ordering matches chronological
	// ordering. Since we want DESC order, we use (maxTime - timestamp).
	maxTime := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
	inverted := maxTime.Sub(timestamp)
	// Format as a fixed-width string so lexicographic order matches
	// the inverted duration (larger durations = older entries, sort first)
	return []byte(fmt.Sprintf("audit:%020d:%s", inverted, id))
}

func auditByAdminKey(adminID, timestampUUID string) []byte {
	return []byte("audit_admin:" + adminID + ":" + timestampUUID)
}

func auditByActionKey(action, timestampUUID string) []byte {
	return []byte("audit_action:" + action + ":" + timestampUUID)
}

func auditByTargetKey(targetType, targetID, timestampUUID string) []byte {
	return []byte("audit_target:" + targetType + ":" + targetID + ":" + timestampUUID)
}

func auditPrefix() []byte {
	return []byte("audit:")
}

func auditByAdminPrefix(adminID string) []byte {
	return []byte("audit_admin:" + adminID + ":")
}

func auditByActionPrefix(action string) []byte {
	return []byte("audit_action:" + action + ":")
}

func auditByTargetPrefix(targetType, targetID string) []byte {
	return []byte("audit_target:" + targetType + ":" + targetID + ":")
}

// keyUpperBound returns an exclusive upper bound key for prefix iteration.
func auditKeyUpperBound(prefix []byte) []byte {
	upper := make([]byte, len(prefix))
	copy(upper, prefix)
	for i := len(prefix) - 1; i >= 0; i-- {
		if prefix[i] < 0xff {
			upper[i]++
			return upper[:i+1]
		}
	}
	return append(prefix, 0x00)
}

// Log records a new audit entry.
func (a *AuditStore) Log(entry *AuditEntry) error {
	if entry.ID == "" {
		entry.ID = uuid.New().String()
	}
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("audit: marshal entry: %w", err)
	}

	tsUUID := fmt.Sprintf("%020d:%s", entry.Timestamp.UnixNano(), entry.ID)

	// Primary key
	if err := a.db.Set(auditEntryKey(entry.Timestamp, entry.ID), data, pebble.Sync); err != nil {
		return fmt.Errorf("audit: save entry: %w", err)
	}

	// Index by admin
	if entry.AdminID != "" {
		if err := a.db.Set(auditByAdminKey(entry.AdminID, tsUUID), []byte(entry.ID), pebble.Sync); err != nil {
			return fmt.Errorf("audit: save admin index: %w", err)
		}
	}

	// Index by action
	if entry.Action != "" {
		if err := a.db.Set(auditByActionKey(entry.Action, tsUUID), []byte(entry.ID), pebble.Sync); err != nil {
			return fmt.Errorf("audit: save action index: %w", err)
		}
	}

	// Index by target
	if entry.TargetType != "" && entry.TargetID != "" {
		if err := a.db.Set(auditByTargetKey(entry.TargetType, entry.TargetID, tsUUID), []byte(entry.ID), pebble.Sync); err != nil {
			return fmt.Errorf("audit: save target index: %w", err)
		}
	}

	return nil
}

// readEntry reads an audit entry by ID from the primary key store.
// It scans the audit: prefix to find the entry by the UUID suffix.
func (a *AuditStore) readEntry(id string) (*AuditEntry, error) {
	prefix := auditPrefix()
	iter, err := a.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: auditKeyUpperBound(prefix),
	})
	if err != nil {
		return nil, fmt.Errorf("audit: read entry iter: %w", err)
	}
	defer iter.Close()

	for iter.First(); iter.Valid(); iter.Next() {
		var entry AuditEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if entry.ID == id {
			return &entry, nil
		}
	}
	return nil, fmt.Errorf("audit: entry %q not found", id)
}

// iteratePrefix reads all entries under the given index prefix and resolves
// them to full AuditEntry objects.
func (a *AuditStore) iteratePrefix(prefix []byte, limit, offset int) ([]*AuditEntry, int, error) {
	// First, collect entry IDs from the index
	var entryIDs []string
	iter, err := a.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: auditKeyUpperBound(prefix),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("audit: iterate index: %w", err)
	}
	defer iter.Close()

	// Reverse iteration for DESC order (newest first)
	// Index keys are ordered by timestamp ascending, so Last/Prev gives newest first
	for iter.Last(); iter.Valid(); iter.Prev() {
		entryID := string(iter.Value())
		entryIDs = append(entryIDs, entryID)
	}
	if err := iter.Error(); err != nil {
		return nil, 0, fmt.Errorf("audit: iterate error: %w", err)
	}

	total := len(entryIDs)

	// Apply offset and limit; clamp negative offset to 0
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*AuditEntry{}, total, nil
	}
	end := offset + limit
	if limit <= 0 || end > total {
		end = total
	}
	entryIDs = entryIDs[offset:end]

	// Resolve entries
	var entries []*AuditEntry
	for _, id := range entryIDs {
		entry, err := a.readEntry(id)
		if err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	return entries, total, nil
}

// List returns audit entries with pagination and optional filters.
// Supported filter keys: "admin_id", "action", "target_type", "target_id".
func (a *AuditStore) List(limit, offset int, filters map[string]string) ([]*AuditEntry, int, error) {
	if filters == nil {
		filters = make(map[string]string)
	}

	adminID := filters["admin_id"]
	action := filters["action"]
	targetType := filters["target_type"]
	targetID := filters["target_id"]

	// If we have specific filters, use the appropriate index
	switch {
	case adminID != "":
		return a.ListByAdmin(adminID, limit, offset)
	case action != "":
		return a.ListByAction(action, limit, offset)
	case targetType != "" && targetID != "":
		return a.ListByTarget(targetType, targetID, limit, offset)
	default:
		// No filters — iterate the primary audit: prefix forward
		// Primary keys use inverted timestamps (newer = smaller key),
		// so forward iteration gives newest first.
		prefix := auditPrefix()
		iter, err := a.db.NewIter(&pebble.IterOptions{
			LowerBound: prefix,
			UpperBound: auditKeyUpperBound(prefix),
		})
		if err != nil {
			return nil, 0, fmt.Errorf("audit: list iter: %w", err)
		}
		defer iter.Close()

		// Collect all entries in forward order (newest first due to inverted keys)
		var allEntries []*AuditEntry
		for iter.First(); iter.Valid(); iter.Next() {
			var entry AuditEntry
			if err := json.Unmarshal(iter.Value(), &entry); err != nil {
				continue
			}
			allEntries = append(allEntries, &entry)
		}
		if err := iter.Error(); err != nil {
			return nil, 0, fmt.Errorf("audit: iterate error: %w", err)
		}

		total := len(allEntries)

		if offset < 0 {
			offset = 0
		}
		if offset >= total {
			return []*AuditEntry{}, total, nil
		}
		end := offset + limit
		if limit <= 0 || end > total {
			end = total
		}
		return allEntries[offset:end], total, nil
	}
}

// ListByAdmin returns audit entries for a specific admin, newest first.
// It also returns the total count (ignoring pagination).
func (a *AuditStore) ListByAdmin(adminID string, limit, offset int) ([]*AuditEntry, int, error) {
	prefix := auditByAdminPrefix(adminID)
	return a.iteratePrefix(prefix, limit, offset)
}

// ListByAction returns audit entries for a specific action, newest first.
// It also returns the total count (ignoring pagination).
func (a *AuditStore) ListByAction(action string, limit, offset int) ([]*AuditEntry, int, error) {
	prefix := auditByActionPrefix(action)
	return a.iteratePrefix(prefix, limit, offset)
}

// ListByTarget returns audit entries for a specific target, newest first.
// It also returns the total count (ignoring pagination).
func (a *AuditStore) ListByTarget(targetType, targetID string, limit, offset int) ([]*AuditEntry, int, error) {
	prefix := auditByTargetPrefix(targetType, targetID)
	return a.iteratePrefix(prefix, limit, offset)
}

// PurgeBefore deletes all audit entries older than the given time.
func (a *AuditStore) PurgeBefore(t time.Time) error {
	prefix := auditPrefix()
	iter, err := a.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: auditKeyUpperBound(prefix),
	})
	if err != nil {
		return fmt.Errorf("audit: purge iter: %w", err)
	}
	defer iter.Close()

	// Collect keys to delete
	var primaryKeys [][]byte
	var indexKeys [][]byte

	for iter.First(); iter.Valid(); iter.Next() {
		var entry AuditEntry
		if err := json.Unmarshal(iter.Value(), &entry); err != nil {
			continue
		}
		if entry.Timestamp.Before(t) {
			primaryKeys = append(primaryKeys, append([]byte{}, iter.Key()...))

			tsUUID := fmt.Sprintf("%020d:%s", entry.Timestamp.UnixNano(), entry.ID)

			if entry.AdminID != "" {
				indexKeys = append(indexKeys, auditByAdminKey(entry.AdminID, tsUUID))
			}
			if entry.Action != "" {
				indexKeys = append(indexKeys, auditByActionKey(entry.Action, tsUUID))
			}
			if entry.TargetType != "" && entry.TargetID != "" {
				indexKeys = append(indexKeys, auditByTargetKey(entry.TargetType, entry.TargetID, tsUUID))
			}
		}
	}
	if err := iter.Error(); err != nil {
		return fmt.Errorf("audit: purge iterate error: %w", err)
	}

	// Delete in batch
	b := a.db.NewBatch()
	defer b.Close()

	for _, k := range primaryKeys {
		if err := b.Delete(k, pebble.Sync); err != nil {
			return fmt.Errorf("audit: purge delete primary: %w", err)
		}
	}
	for _, k := range indexKeys {
		if err := b.Delete(k, pebble.Sync); err != nil {
			return fmt.Errorf("audit: purge delete index: %w", err)
		}
	}

	return b.Commit(pebble.Sync)
}

// Count returns the total number of audit entries.
func (a *AuditStore) Count() (int, error) {
	prefix := auditPrefix()
	iter, err := a.db.NewIter(&pebble.IterOptions{
		LowerBound: prefix,
		UpperBound: auditKeyUpperBound(prefix),
	})
	if err != nil {
		return 0, fmt.Errorf("audit: count iter: %w", err)
	}
	defer iter.Close()

	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	return count, iter.Error()
}

// Helper to sort audit entries by timestamp descending.
func sortEntriesDesc(entries []*AuditEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})
}
