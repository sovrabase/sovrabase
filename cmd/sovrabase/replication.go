package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/db"
	"github.com/ketsuna-org/sovrabase/internal/replication"
)

// startReplication initializes the replication subsystem for this node.
//
// Architecture:
//   - Master: starts a StreamServer to push log entries to Readers, and a
//     LeaseManager to heartbeat and track peers.
//   - Heir: connects to Master as a StreamClient, applies entries to local
//     engine. If Master dies, promotes to Master.
//   - Reader: connects to Master as a StreamClient, applies entries locally,
//     stays read-only.
func startReplication(ctx context.Context, cfg *config.Config, engine *db.Engine) (*replication.Node, error) {
	logger := slog.Default()

	nodeCfg := &replication.NodeConfig{
		NodeID:     cfg.NodeID,
		ListenAddr: cfg.ReplAddr,
		Role:       replication.Role(cfg.Role),
		Peers:      cfg.Peers,
		DataDir:    cfg.DataDir,
		LeaseTTL:   cfg.LeaseTTL,
	}

	// Create the Write-Ahead Log
	wal, err := replication.NewReplicationLog(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("create WAL: %w", err)
	}

	// Create the replication node
	node := replication.NewNode(nodeCfg, engine)
	node.SetLog(wal)

	// Determine initial role and start
	switch nodeCfg.Role {
	case replication.RoleMaster:
		if err := node.BecomeMaster(); err != nil {
			return nil, fmt.Errorf("become master: %w", err)
		}

		// Start streaming server for Readers to connect
		streamServer := replication.NewStreamServer(nodeCfg.ListenAddr)
		streamServer.SetLogProvider(func(lsn uint64) (<-chan *replication.LogEntry, context.CancelFunc) {
			ch, _, cancel := wal.StreamFrom(lsn)
			return ch, cancel
		})

		go func() {
			if err := streamServer.Start(ctx); err != nil {
				logger.Error("Stream server error", "error", err)
			}
		}()

		// Start lease heartbeat
		leaseMgr := replication.NewLeaseManager(node)
		go leaseMgr.Start(ctx)

		logger.Info("Master node ready",
			"repl_addr", nodeCfg.ListenAddr,
			"peers", len(nodeCfg.Peers),
		)

	case replication.RoleHeir, replication.RoleReader:
		if err := node.BecomeReader(); err != nil {
			return nil, fmt.Errorf("become reader: %w", err)
		}
		if nodeCfg.Role == replication.RoleHeir {
			_ = node.BecomeHeir() // mark as heir candidate
		}

		if len(nodeCfg.Peers) == 0 {
			return nil, fmt.Errorf("reader node requires at least one peer (master address)")
		}

		// Connect to master
		masterAddr := nodeCfg.Peers[0]
		client := replication.NewStreamClient(masterAddr, nodeCfg.NodeID, 0)
		if err := client.Connect(ctx); err != nil {
			return nil, fmt.Errorf("connect to master at %s: %w", masterAddr, err)
		}

		// Enable auto-reconnect after initial connection succeeds
		client.SetReconnect(true)

		// For Heir nodes, register a promotion callback that starts the
		// StreamServer and LeaseManager when this node becomes Master.
		if nodeCfg.Role == replication.RoleHeir {
			heirClient := client // capture for closure
			node.SetOnPromote(func() {
				// Close the old client — no longer reading from the former Master.
				heirClient.Close()

				// Start StreamServer so Readers can connect to this promoted Master.
				newStreamServer := replication.NewStreamServer(nodeCfg.ListenAddr)
				newStreamServer.SetLogProvider(func(lsn uint64) (<-chan *replication.LogEntry, context.CancelFunc) {
					ch, _, cancel := wal.StreamFrom(lsn)
					return ch, cancel
				})
				go func() {
					if err := newStreamServer.Start(ctx); err != nil {
						logger.Error("Stream server error after promotion", "error", err)
					}
				}()

				// Start a new LeaseManager for heartbeats as Master.
				newLeaseMgr := replication.NewLeaseManager(node)
				go func() {
					if err := newLeaseMgr.Start(ctx); err != nil {
						logger.Error("Lease manager error after promotion", "error", err)
					}
				}()

				logger.Info("Promoted Heir: StreamServer and LeaseManager started")
			})
		}

		// Apply incoming entries
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case entry, ok := <-client.Entries():
					if !ok {
						logger.Warn("Replication stream closed, reconnecting...")
						time.Sleep(time.Second)
						_ = client.Reconnect(ctx)
						continue
					}
					if err := node.ApplyEntry(entry); err != nil {
						logger.Error("Failed to apply replicated entry",
							"lsn", entry.LSN,
							"error", err,
						)
					}
				case err, ok := <-client.Errors():
					if !ok {
						return
					}
					logger.Error("Replication stream error", "error", err)
				}
			}
		}()

		// If Heir, also monitor Master health for failover
		if nodeCfg.Role == replication.RoleHeir {
			go monitorMasterHealth(ctx, node, cfg.NodeID, cfg.LeaseTTL, client)
		}

		logger.Info("Replication node ready",
			"role", nodeCfg.Role,
			"master", masterAddr,
		)

	default:
		return nil, fmt.Errorf("unknown replication role: %s", nodeCfg.Role)
	}

	return node, nil
}

// monitorMasterHealth watches for Master failure and promotes Heir to Master.
// It uses the StreamClient's last-received-time to detect liveness without
// consuming log entries from the Entries channel (which belong to the apply loop).
func monitorMasterHealth(ctx context.Context, node *replication.Node, nodeID string, leaseTTL time.Duration, client *replication.StreamClient) {
	logger := slog.Default()
	ticker := time.NewTicker(leaseTTL)
	defer ticker.Stop()

	failures := 0
	maxFailures := 3

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check liveness via last successful WebSocket read time instead of
			// consuming from client.Entries() — entries must stay in the channel
			// for the apply goroutine.
			lastRecv := client.LastRecvTime()
			if lastRecv.IsZero() || time.Since(lastRecv) > leaseTTL*3 {
				failures++
			} else {
				failures = 0
			}

			if failures >= maxFailures {
				logger.Warn("Master appears dead, promoting Heir to Master",
					"node_id", nodeID,
					"failures", failures,
				)
				// node.BecomeMaster() triggers the onPromote callback set in
				// startReplication, which starts StreamServer + LeaseManager.
				if err := node.BecomeMaster(); err != nil {
					logger.Error("Failed to promote to Master", "error", err)
				} else {
					logger.Info("Heir promoted to Master successfully")
					return // stop monitoring once promoted
				}
			}
		}
	}
}
