// Package replication implements the Single-Master / Master-Heir / Multi-Reader
// high-availability architecture for Sovrabase.
package replication

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/db"
)

// Node is a replication-aware database node that participates in the
// Single-Master / Master-Heir / Multi-Reader topology.
type Node struct {
	engine       *db.Engine
	role         Role
	config       *NodeConfig
	log          *ReplicationLog
	leaseManager *LeaseManager

	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	lsnCounter uint64 // monotonically increasing, atomic

	onPromote func() // called after successful promotion to Master (Heir→Master)
}

// NewNode creates a new replication Node.
func NewNode(cfg *NodeConfig, engine *db.Engine) *Node {
	if cfg == nil {
		cfg = DefaultNodeConfig()
	}
	return &Node{
		engine: engine,
		role:   cfg.Role,
		config: cfg,
	}
}

// Start starts the node in its configured role.
// It initialises the lease manager for Heir/Reader nodes that have peers.
func (n *Node) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.ctx, n.cancel = context.WithCancel(ctx)

	// Only Heir and Reader nodes participate in lease-based election.
	if len(n.config.Peers) > 0 && n.role != RoleMaster {
		n.leaseManager = NewLeaseManager(n)
		if err := n.leaseManager.Start(n.ctx); err != nil {
			return fmt.Errorf("replication: start lease manager: %w", err)
		}
	}

	return nil
}

// Stop performs a graceful shutdown of the node, stopping the lease manager
// and closing the replication log if set.
func (n *Node) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.cancel != nil {
		n.cancel()
	}

	if n.leaseManager != nil {
		n.leaseManager.Stop()
		n.leaseManager = nil
	}

	if n.log != nil {
		_ = n.log.Close()
	}

	return nil
}

// Role returns the node's current operational role.
func (n *Node) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

// SetLog assigns a replication log to the node.
// This is set externally after construction when a WAL is available.
func (n *Node) SetLog(log *ReplicationLog) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.log = log
}

// SetOnPromote registers a callback that is invoked after the node
// successfully transitions from Heir to Master. The callback runs
// while the write lock is held.
func (n *Node) SetOnPromote(fn func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.onPromote = fn
}

// BecomeMaster transitions the node to the Master role.
// It stops lease monitoring since the Master does not participate in
// lease-based election.
func (n *Node) BecomeMaster() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.becomeMasterLocked()
}

// becomeMasterLocked is the internal helper that assumes the lock is held.
func (n *Node) becomeMasterLocked() error {
	if n.role == RoleMaster {
		return nil
	}
	n.role = RoleMaster

	// Stop the lease manager: the Master is the leader itself.
	if n.leaseManager != nil {
		n.leaseManager.Stop()
		n.leaseManager = nil
	}

	// Invoke the promotion callback if set (e.g. to start StreamServer).
	if n.onPromote != nil {
		n.onPromote()
	}
	return nil
}

// BecomeHeir transitions the node to the Heir role.
func (n *Node) BecomeHeir() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role == RoleHeir {
		return nil
	}
	n.role = RoleHeir

	// Ensure lease manager is running for Heir.
	if n.leaseManager == nil && len(n.config.Peers) > 0 && n.ctx != nil {
		n.leaseManager = NewLeaseManager(n)
		return n.leaseManager.Start(n.ctx)
	}
	return nil
}

// BecomeReader transitions the node to the Reader role.
func (n *Node) BecomeReader() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.role = RoleReader
	return nil
}

// ApplyEntry applies a replicated log entry to the local engine.
// It deserialises the entry's Data field and dispatches to the appropriate
// engine method based on the operation type.
func (n *Node) ApplyEntry(entry *LogEntry) error {
	if entry == nil {
		return fmt.Errorf("replication: nil log entry")
	}

	switch entry.Operation {
	case OpCreateCollection:
		return n.engine.CreateCollection(entry.Collection)

	case OpDropCollection:
		return n.engine.DropCollection(entry.Collection)

	case OpInsert:
		doc, err := unmarshalDoc(entry.Data)
		if err != nil {
			return fmt.Errorf("replication: insert unmarshal: %w", err)
		}
		return n.engine.Insert(entry.Collection, entry.DocID, doc)

	case OpUpdate:
		doc, err := unmarshalDoc(entry.Data)
		if err != nil {
			return fmt.Errorf("replication: update unmarshal: %w", err)
		}
		return n.engine.Update(entry.Collection, entry.DocID, doc)

	case OpDelete:
		return n.engine.Delete(entry.Collection, entry.DocID)

	default:
		return fmt.Errorf("replication: unknown operation %q", entry.Operation)
	}
}

// Write performs a write operation on the Master node.
// It first applies the mutation to the local engine, then appends a log entry
// to the replication log so that Heir/Reader nodes can replay it.
// Returns the assigned LSN.
func (n *Node) Write(collection string, op Operation, id string, doc map[string]interface{}) (uint64, error) {
	n.mu.RLock()
	role := n.role
	n.mu.RUnlock()

	if role != RoleMaster {
		return 0, fmt.Errorf("replication: write denied: node is %s, only master accepts writes", role)
	}

	// Apply to local engine first.
	switch op {
	case OpCreateCollection:
		if err := n.engine.CreateCollection(collection); err != nil {
			return 0, fmt.Errorf("replication: write create_collection: %w", err)
		}
	case OpDropCollection:
		if err := n.engine.DropCollection(collection); err != nil {
			return 0, fmt.Errorf("replication: write drop_collection: %w", err)
		}
	case OpInsert:
		if err := n.engine.Insert(collection, id, doc); err != nil {
			return 0, fmt.Errorf("replication: write insert: %w", err)
		}
	case OpUpdate:
		if err := n.engine.Update(collection, id, doc); err != nil {
			return 0, fmt.Errorf("replication: write update: %w", err)
		}
	case OpDelete:
		if err := n.engine.Delete(collection, id); err != nil {
			return 0, fmt.Errorf("replication: write delete: %w", err)
		}
	default:
		return 0, fmt.Errorf("replication: write unknown operation %q", op)
	}

	// Increment LSN.
	lsn := atomic.AddUint64(&n.lsnCounter, 1)

	// Serialise document data for the log entry.
	var data []byte
	if doc != nil {
		var err error
		data, err = json.Marshal(doc)
		if err != nil {
			return 0, fmt.Errorf("replication: marshal doc for log: %w", err)
		}
	}

	entry := &LogEntry{
		LSN:        lsn,
		Timestamp:  time.Now().UTC(),
		Operation:  op,
		Collection: collection,
		DocID:      id,
		Data:       data,
	}

	// Append to replication log if available.
	n.mu.RLock()
	log := n.log
	n.mu.RUnlock()

	if log != nil {
		if _, err := log.Append(entry); err != nil {
			return 0, fmt.Errorf("replication: append to log: %w", err)
		}
	}

	return lsn, nil
}

// CurrentLSN returns the current Log Sequence Number counter value.
func (n *Node) CurrentLSN() uint64 {
	return atomic.LoadUint64(&n.lsnCounter)
}

// Config returns a copy of the node's configuration.
func (n *Node) Config() *NodeConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	cp := *n.config
	return &cp
}

// unmarshalDoc deserialises JSON data into a document map.
func unmarshalDoc(data []byte) (map[string]interface{}, error) {
	if len(data) == 0 {
		return nil, nil
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return doc, nil
}
