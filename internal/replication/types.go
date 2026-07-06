// Package replication implements the Single-Master / Master-Heir / Multi-Reader
// high-availability architecture for Sovrabase.
package replication

import (
	"time"
)

// Role represents the operational mode of a node.
type Role string

const (
	RoleMaster Role = "master"
	RoleHeir   Role = "heir"
	RoleReader Role = "reader"
)

// Namespace constants for WAL routing.
const (
	NamespaceGlobal = "global"
	NamespaceMaster = "_master"
)

// NormalizeNamespace returns the canonical namespace for a WAL entry.
// Empty namespace is treated as global for backward compatibility.
func NormalizeNamespace(ns string) string {
	if ns == "" {
		return NamespaceGlobal
	}
	return ns
}

// Operation describes the type of database mutation in a log entry.
type Operation string

const (
	OpCreateCollection Operation = "create_collection"
	OpDropCollection   Operation = "drop_collection"
	OpInsert           Operation = "insert"
	OpUpdate           Operation = "update"
	OpDelete           Operation = "delete"

	// Tenant / metadata operations (namespace _master).
	OpCreateProject Operation = "create_project"
	OpUpdateProject Operation = "update_project"
	OpDeleteProject Operation = "delete_project"

	// Index and RLS operations.
	OpCreateIndex Operation = "create_index"
	OpDropIndex   Operation = "drop_index"
	OpSetRules    Operation = "set_rules"

	// Fencing: incremented on each Master promotion.
	OpMasterEpoch Operation = "master_epoch"
)

// LogEntry is a single entry in the Write-Ahead Log.
type LogEntry struct {
	LSN        uint64    `json:"lsn"`
	Timestamp  time.Time `json:"timestamp"`
	Namespace  string    `json:"namespace,omitempty"`
	Operation  Operation `json:"operation"`
	Collection string    `json:"collection"`
	DocID      string    `json:"doc_id"`
	Data       []byte    `json:"data"`
	MasterEpoch uint64   `json:"master_epoch,omitempty"`
}

// PeerInfo describes a connected peer node.
type PeerInfo struct {
	ID       string    `json:"id"`
	Role     Role      `json:"role"`
	Addr     string    `json:"addr"`
	LastSeen time.Time `json:"last_seen"`
	LastLSN  uint64    `json:"last_lsn"`
}

// Engine is the interface the replication layer needs from the database engine.
type Engine interface {
	CreateCollection(name string) error
	DropCollection(name string) error
	Insert(collection, id string, doc map[string]interface{}) error
	Update(collection, id string, doc map[string]interface{}) error
	Delete(collection, id string) error
}

// TenantApplier applies replicated tenant metadata operations on followers.
type TenantApplier interface {
	ApplyCreateProject(data []byte) error
	ApplyUpdateProject(data []byte) error
	ApplyDeleteProject(projectID string) error
}

// AppliedCallback is invoked after a WAL entry is applied locally.
type AppliedCallback func(entry *LogEntry)

// NodeConfig holds configuration for a replication node.
type NodeConfig struct {
	NodeID     string        `json:"node_id"`
	ListenAddr string        `json:"listen_addr"`
	Role       Role          `json:"role"`
	Peers      []string      `json:"peers"`
	DataDir    string        `json:"data_dir"`
	LeaseTTL   time.Duration `json:"lease_ttl"`
}

// DefaultNodeConfig returns a NodeConfig with sensible defaults.
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		NodeID:     "node-1",
		ListenAddr: ":9090",
		Role:       RoleMaster,
		Peers:      nil,
		LeaseTTL:   5 * time.Second,
	}
}
