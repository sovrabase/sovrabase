// Package replication implements the Single-Master / Master-Heir / Multi-Reader
// high-availability architecture for Sovrabase.
//
// Architecture:
//   - Master: accepts writes, generates a sequential Write-Ahead Log (WAL),
//     streams log entries to all connected Readers via WebSocket.
//   - Heir: a designated Reader that will take over as Master if the current
//     Master becomes unavailable (lease-based failover).
//   - Reader: read-only replica that applies WAL entries to its local Pebble
//     instance, staying in sync with the Master.
//
// The replication log uses Log Sequence Numbers (LSNs) — monotonically
// increasing uint64 values. Each log entry is atomic and idempotent.
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

// Operation describes the type of database mutation in a log entry.
type Operation string

const (
	OpCreateCollection Operation = "create_collection"
	OpDropCollection   Operation = "drop_collection"
	OpInsert           Operation = "insert"
	OpUpdate           Operation = "update"
	OpDelete           Operation = "delete"
)

// LogEntry is a single entry in the Write-Ahead Log.
type LogEntry struct {
	LSN        uint64    `json:"lsn"`
	Timestamp  time.Time `json:"timestamp"`
	Operation  Operation `json:"operation"`
	Collection string    `json:"collection"`
	DocID      string    `json:"doc_id"`
	Data       []byte    `json:"data"` // JSON-encoded document (nil for deletes/drops)
}

// PeerInfo describes a connected peer node.
type PeerInfo struct {
	ID        string    `json:"id"`
	Role      Role      `json:"role"`
	Addr      string    `json:"addr"`
	LastSeen  time.Time `json:"last_seen"`
	LastLSN   uint64    `json:"last_lsn"`
}

// Engine is the interface the replication layer needs from the database engine.
// It mirrors db.Engine's mutation methods — db.Engine already satisfies this.
type Engine interface {
	CreateCollection(name string) error
	DropCollection(name string) error
	Insert(collection, id string, doc map[string]interface{}) error
	Update(collection, id string, doc map[string]interface{}) error
	Delete(collection, id string) error
}

// NodeConfig holds configuration for a replication node.
type NodeConfig struct {
	NodeID     string        `json:"node_id"`
	ListenAddr string        `json:"listen_addr"` // WebSocket listen address
	Role       Role          `json:"role"`
	Peers      []string      `json:"peers"`       // addresses of other nodes
	DataDir    string        `json:"data_dir"`
	LeaseTTL   time.Duration `json:"lease_ttl"`   // how long a lease is valid
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
