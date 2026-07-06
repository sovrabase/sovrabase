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

// ClusterNode coordinates multi-namespace WAL replication across Pebble engines.
type ClusterNode struct {
	engines       map[string]*db.Engine
	engineOpener  func(projectID string) (*db.Engine, error)
	tenantApplier TenantApplier
	onApplied     AppliedCallback

	role         Role
	config       *NodeConfig
	log          *ReplicationLog
	leaseManager *LeaseManager

	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	lsnCounter  uint64
	masterEpoch uint64
	lastApplied uint64

	onPromote func()
}

// Node is an alias for ClusterNode (backward compatibility).
type Node = ClusterNode

// NewClusterNode creates a replication node with an empty engine registry.
func NewClusterNode(cfg *NodeConfig) *ClusterNode {
	if cfg == nil {
		cfg = DefaultNodeConfig()
	}
	return &ClusterNode{
		engines: make(map[string]*db.Engine),
		role:    cfg.Role,
		config:  cfg,
		masterEpoch: func() uint64 {
			if cfg.Role == RoleMaster {
				return 1
			}
			return 0
		}(),
	}
}

// NewNode creates a ClusterNode with a single global engine registered.
func NewNode(cfg *NodeConfig, engine *db.Engine) *ClusterNode {
	cn := NewClusterNode(cfg)
	if engine != nil {
		cn.RegisterEngine(NamespaceGlobal, engine)
	}
	return cn
}

// RegisterEngine binds a namespace to a Pebble engine.
func (n *ClusterNode) RegisterEngine(namespace string, engine *db.Engine) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.engines[NormalizeNamespace(namespace)] = engine
}

// SetEngineOpener sets a callback used to lazily open project engines.
func (n *ClusterNode) SetEngineOpener(fn func(projectID string) (*db.Engine, error)) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.engineOpener = fn
}

// SetTenantApplier sets the handler for _master namespace tenant ops.
func (n *ClusterNode) SetTenantApplier(ta TenantApplier) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.tenantApplier = ta
}

// SetOnApplied registers a callback invoked after each WAL entry is applied.
func (n *ClusterNode) SetOnApplied(fn AppliedCallback) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.onApplied = fn
}

func (n *ClusterNode) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.ctx, n.cancel = context.WithCancel(ctx)

	if len(n.config.Peers) > 0 && n.role != RoleMaster {
		n.leaseManager = NewLeaseManager(n)
		if err := n.leaseManager.Start(n.ctx); err != nil {
			return fmt.Errorf("replication: start lease manager: %w", err)
		}
	}
	return nil
}

func (n *ClusterNode) Stop() error {
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

func (n *ClusterNode) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

func (n *ClusterNode) SetLog(log *ReplicationLog) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.log = log
	if log != nil {
		n.lsnCounter = log.CurrentLSN()
	}
}

func (n *ClusterNode) SetOnPromote(fn func()) {
	n.mu.Lock()
	defer n.mu.Unlock()
	prev := n.onPromote
	n.onPromote = func() {
		if prev != nil {
			prev()
		}
		if fn != nil {
			fn()
		}
	}
}

func (n *ClusterNode) BecomeMaster() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.becomeMasterLocked()
}

func (n *ClusterNode) becomeMasterLocked() error {
	if n.role == RoleMaster {
		return nil
	}
	n.role = RoleMaster
	n.masterEpoch++

	if n.leaseManager != nil {
		n.leaseManager.Stop()
		n.leaseManager = nil
	}

	// Record fencing epoch in WAL (best-effort on promotion).
	if n.log != nil {
		lsn := atomic.AddUint64(&n.lsnCounter, 1)
		entry := &LogEntry{
			LSN:         lsn,
			Timestamp:   time.Now().UTC(),
			Namespace:   NamespaceGlobal,
			Operation:   OpMasterEpoch,
			MasterEpoch: n.masterEpoch,
		}
		_, _ = n.log.Append(entry)
	}

	if n.onPromote != nil {
		n.onPromote()
	}
	return nil
}

func (n *ClusterNode) BecomeHeir() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.role == RoleHeir {
		return nil
	}
	n.role = RoleHeir

	if n.leaseManager == nil && len(n.config.Peers) > 0 && n.ctx != nil {
		n.leaseManager = NewLeaseManager(n)
		return n.leaseManager.Start(n.ctx)
	}
	return nil
}

func (n *ClusterNode) BecomeReader() error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.role = RoleReader
	return nil
}

func (n *ClusterNode) MasterEpoch() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.masterEpoch
}

func (n *ClusterNode) CurrentLSN() uint64 {
	return atomic.LoadUint64(&n.lsnCounter)
}

func (n *ClusterNode) LastAppliedLSN() uint64 {
	return atomic.LoadUint64(&n.lastApplied)
}

func (n *ClusterNode) ReplicationLag() uint64 {
	current := atomic.LoadUint64(&n.lsnCounter)
	applied := atomic.LoadUint64(&n.lastApplied)
	if current > applied {
		return current - applied
	}
	return 0
}

func (n *ClusterNode) Config() *NodeConfig {
	n.mu.RLock()
	defer n.mu.RUnlock()
	cp := *n.config
	return &cp
}

// DefaultEngine returns the global namespace engine (for tests and diagnostics).
func (n *ClusterNode) DefaultEngine() *db.Engine {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.engines[NamespaceGlobal]
}

// WriteNS performs a replicated write in the given namespace (Master only).
func (n *ClusterNode) WriteNS(namespace, collection string, op Operation, id string, doc map[string]interface{}) (uint64, error) {
	return n.writeNS(namespace, collection, op, id, doc, 0)
}

// Write performs a replicated write in the global namespace (backward compat).
func (n *ClusterNode) Write(collection string, op Operation, id string, doc map[string]interface{}) (uint64, error) {
	return n.WriteNS(NamespaceGlobal, collection, op, id, doc)
}

func (n *ClusterNode) writeNS(namespace, collection string, op Operation, id string, doc map[string]interface{}, epoch uint64) (uint64, error) {
	n.mu.RLock()
	role := n.role
	currentEpoch := n.masterEpoch
	n.mu.RUnlock()

	if role != RoleMaster {
		return 0, fmt.Errorf("replication: write denied: node is %s, only master accepts writes", role)
	}
	if epoch > 0 && epoch < currentEpoch {
		return 0, fmt.Errorf("replication: stale master epoch %d (current %d)", epoch, currentEpoch)
	}

	ns := NormalizeNamespace(namespace)

	if err := n.applyMutation(ns, collection, op, id, doc); err != nil {
		return 0, err
	}

	lsn := atomic.AddUint64(&n.lsnCounter, 1)
	atomic.StoreUint64(&n.lastApplied, lsn)

	var data []byte
	if doc != nil {
		var err error
		data, err = json.Marshal(doc)
		if err != nil {
			return 0, fmt.Errorf("replication: marshal doc for log: %w", err)
		}
	}

	entry := &LogEntry{
		LSN:         lsn,
		Timestamp:   time.Now().UTC(),
		Namespace:   ns,
		Operation:   op,
		Collection:  collection,
		DocID:       id,
		Data:        data,
		MasterEpoch: currentEpoch,
	}

	n.mu.RLock()
	log := n.log
	onApplied := n.onApplied
	n.mu.RUnlock()

	if log != nil {
		if _, err := log.Append(entry); err != nil {
			return 0, fmt.Errorf("replication: append to log: %w", err)
		}
	}

	if onApplied != nil {
		onApplied(entry)
	}

	return lsn, nil
}

// ApplyEntry applies a replicated WAL entry on this node.
func (n *ClusterNode) ApplyEntry(entry *LogEntry) error {
	if entry == nil {
		return fmt.Errorf("replication: nil log entry")
	}

	ns := NormalizeNamespace(entry.Namespace)

	switch entry.Operation {
	case OpMasterEpoch:
		n.mu.Lock()
		if entry.MasterEpoch > n.masterEpoch {
			n.masterEpoch = entry.MasterEpoch
		}
		n.mu.Unlock()
		atomic.StoreUint64(&n.lastApplied, entry.LSN)
		return nil

	case OpCreateProject, OpUpdateProject, OpDeleteProject:
		n.mu.RLock()
		ta := n.tenantApplier
		n.mu.RUnlock()
		if ta == nil {
			return fmt.Errorf("replication: no tenant applier for %s", entry.Operation)
		}
		var err error
		switch entry.Operation {
		case OpCreateProject:
			err = ta.ApplyCreateProject(entry.Data)
		case OpUpdateProject:
			err = ta.ApplyUpdateProject(entry.Data)
		case OpDeleteProject:
			err = ta.ApplyDeleteProject(entry.DocID)
		}
		if err != nil {
			return err
		}
		atomic.StoreUint64(&n.lastApplied, entry.LSN)
		n.fireOnApplied(entry)
		return nil
	}

	if err := n.applyMutation(ns, entry.Collection, entry.Operation, entry.DocID, entry.Data); err != nil {
		return err
	}

	atomic.StoreUint64(&n.lastApplied, entry.LSN)
	n.fireOnApplied(entry)
	return nil
}

func (n *ClusterNode) fireOnApplied(entry *LogEntry) {
	n.mu.RLock()
	fn := n.onApplied
	n.mu.RUnlock()
	if fn != nil {
		fn(entry)
	}
}

func (n *ClusterNode) applyMutation(namespace, collection string, op Operation, id string, data interface{}) error {
	switch op {
	case OpCreateProject, OpUpdateProject, OpDeleteProject, OpMasterEpoch:
		return nil // handled elsewhere
	}

	engine, err := n.resolveEngine(namespace)
	if err != nil {
		return err
	}

	switch op {
	case OpCreateCollection:
		return engine.CreateCollection(collection)
	case OpDropCollection:
		return engine.DropCollection(collection)
	case OpInsert:
		doc, err := unmarshalDocData(data)
		if err != nil {
			return fmt.Errorf("replication: insert unmarshal: %w", err)
		}
		return engine.Insert(collection, id, doc)
	case OpUpdate:
		doc, err := unmarshalDocData(data)
		if err != nil {
			return fmt.Errorf("replication: update unmarshal: %w", err)
		}
		return engine.Update(collection, id, doc)
	case OpDelete:
		return engine.Delete(collection, id)
	case OpCreateIndex:
		var idx indexOpData
		if err := unmarshalJSONData(data, &idx); err != nil {
			return fmt.Errorf("replication: create_index unmarshal: %w", err)
		}
		return engine.CreateIndex(idx.Collection, idx.Field, db.IndexType(idx.IndexType))
	case OpDropIndex:
		var idx indexOpData
		if err := unmarshalJSONData(data, &idx); err != nil {
			return fmt.Errorf("replication: drop_index unmarshal: %w", err)
		}
		return engine.DropIndex(idx.Collection, idx.Field)
	case OpSetRules:
		var rules rulesOpData
		if err := unmarshalJSONData(data, &rules); err != nil {
			return fmt.Errorf("replication: set_rules unmarshal: %w", err)
		}
		return engine.SetRules(rules.Collection, &rules.Rules)
	default:
		return fmt.Errorf("replication: unknown operation %q", op)
	}
}

func (n *ClusterNode) resolveEngine(namespace string) (*db.Engine, error) {
	ns := NormalizeNamespace(namespace)

	n.mu.RLock()
	engine, ok := n.engines[ns]
	opener := n.engineOpener
	n.mu.RUnlock()

	if ok && engine != nil {
		return engine, nil
	}

	// Lazy-open project engines.
	if ns != NamespaceGlobal && ns != NamespaceMaster && opener != nil {
		opened, err := opener(ns)
		if err != nil {
			return nil, fmt.Errorf("replication: open engine %q: %w", ns, err)
		}
		n.RegisterEngine(ns, opened)
		return opened, nil
	}

	return nil, fmt.Errorf("replication: no engine for namespace %q", ns)
}

type indexOpData struct {
	Collection string `json:"collection"`
	Field      string `json:"field"`
	IndexType  string `json:"index_type"`
}

type rulesOpData struct {
	Collection string          `json:"collection"`
	Rules      db.RulesConfig  `json:"rules"`
}

func unmarshalDocData(data interface{}) (map[string]interface{}, error) {
	switch v := data.(type) {
	case []byte:
		return unmarshalDoc(v)
	case map[string]interface{}:
		return v, nil
	case nil:
		return nil, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		return unmarshalDoc(b)
	}
}

func unmarshalJSONData(data interface{}, dest interface{}) error {
	switch v := data.(type) {
	case []byte:
		return json.Unmarshal(v, dest)
	case nil:
		return nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, dest)
	}
}

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

func (n *ClusterNode) WriteCreateIndex(namespace, collection, field string, idxType db.IndexType) error {
	data, err := json.Marshal(indexOpData{Collection: collection, Field: field, IndexType: string(idxType)})
	if err != nil {
		return err
	}
	return n.writeRaw(namespace, collection, OpCreateIndex, field, data, false)
}

func (n *ClusterNode) WriteDropIndex(namespace, collection, field string) error {
	data, err := json.Marshal(indexOpData{Collection: collection, Field: field})
	if err != nil {
		return err
	}
	return n.writeRaw(namespace, collection, OpDropIndex, field, data, false)
}

func (n *ClusterNode) WriteSetRules(namespace, collection string, cfg *db.RulesConfig) error {
	data, err := json.Marshal(rulesOpData{Collection: collection, Rules: *cfg})
	if err != nil {
		return err
	}
	return n.writeRaw(namespace, collection, OpSetRules, collection, data, false)
}

func (n *ClusterNode) WriteTenantOp(op Operation, projectID string, data []byte) error {
	return n.writeRaw(NamespaceMaster, "", op, projectID, data, true)
}

func (n *ClusterNode) writeRaw(namespace, collection string, op Operation, id string, data []byte, skipApply bool) error {
	n.mu.RLock()
	role := n.role
	currentEpoch := n.masterEpoch
	n.mu.RUnlock()

	if role != RoleMaster {
		return fmt.Errorf("replication: write denied: node is %s, only master accepts writes", role)
	}

	ns := NormalizeNamespace(namespace)

	if !skipApply {
		if err := n.applyMutation(ns, collection, op, id, data); err != nil {
			return err
		}
	}

	lsn := atomic.AddUint64(&n.lsnCounter, 1)
	atomic.StoreUint64(&n.lastApplied, lsn)

	entry := &LogEntry{
		LSN:         lsn,
		Timestamp:   time.Now().UTC(),
		Namespace:   ns,
		Operation:   op,
		Collection:  collection,
		DocID:       id,
		Data:        data,
		MasterEpoch: currentEpoch,
	}

	n.mu.RLock()
	log := n.log
	n.mu.RUnlock()

	if log != nil {
		if _, err := log.Append(entry); err != nil {
			return fmt.Errorf("replication: append to log: %w", err)
		}
	}
	n.fireOnApplied(entry)
	return nil
}
