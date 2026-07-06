package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/api"
	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/metering"
	"github.com/ketsuna-org/sovrabase/internal/realtime"
	"github.com/ketsuna-org/sovrabase/internal/replication"
	"github.com/ketsuna-org/sovrabase/internal/tenant"
)

// setupReplicationRealtime publishes DB mutation events from the WAL apply path.
func setupReplicationRealtime(hub *realtime.Hub, cluster *replication.ClusterNode) {
	cluster.SetOnApplied(func(entry *replication.LogEntry) {
		if hub == nil || entry == nil {
			return
		}

		var eventType realtime.EventType
		switch entry.Operation {
		case replication.OpInsert:
			eventType = realtime.EventInsert
		case replication.OpUpdate:
			eventType = realtime.EventUpdate
		case replication.OpDelete:
			eventType = realtime.EventDelete
		default:
			return
		}

		ns := replication.NormalizeNamespace(entry.Namespace)
		if ns == replication.NamespaceGlobal || ns == replication.NamespaceMaster {
			return
		}

		var data map[string]interface{}
		if len(entry.Data) > 0 {
			_ = json.Unmarshal(entry.Data, &data)
		}

		hub.Publish(&realtime.Event{
			Type:       eventType,
			Collection: entry.Collection,
			DocID:      entry.DocID,
			Data:       data,
			ProjectID:  ns,
			Timestamp:  entry.Timestamp,
		})
	})
}

type masterJobs struct {
	cancel context.CancelFunc
}

// startMasterBackgroundJobs runs cron, functions, and scheduled backups on the active master.
func startMasterBackgroundJobs(
	parent context.Context,
	logger *slog.Logger,
	cfg *config.Config,
	pm *tenant.ProjectManager,
	meterStore *metering.MeterStore,
) *masterJobs {
	ctx, cancel := context.WithCancel(parent)

	api.StartProjectSchedulers(ctx, pm)
	api.RegisterFunctionStores(ctx, pm, meterStore)
	logger.Info("Master background jobs started", "projects", pm.ProjectCount())

	if cfg.BackupInterval > 0 {
		go runBackupScheduler(ctx, logger, cfg, pm)
		logger.Info("Backup scheduler started", "interval", cfg.BackupInterval)
	}

	return &masterJobs{cancel: cancel}
}

func (m *masterJobs) Stop() {
	if m != nil && m.cancel != nil {
		m.cancel()
	}
}

func runBackupScheduler(ctx context.Context, logger *slog.Logger, cfg *config.Config, pm *tenant.ProjectManager) {
	ticker := time.NewTicker(cfg.BackupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			logger.Info("Running scheduled backup...")
			backupDir := filepath.Join(cfg.DataDir, "backups")
			if err := os.MkdirAll(backupDir, 0755); err != nil {
				logger.Error("Failed to create backup directory", "error", err)
				continue
			}
			timestamp := time.Now().UTC().Format("20060102T150405Z")
			backupName := "backup-" + timestamp
			backupPath := filepath.Join(backupDir, backupName)
			if err := pm.Backup(backupPath); err != nil {
				logger.Error("Failed to create backup", "error", err)
				continue
			}
			logger.Info("Scheduled backup completed", "name", backupName, "path", backupPath)
		case <-ctx.Done():
			return
		}
	}
}

func validateHAConfig(cfg *config.Config, logger *slog.Logger) {
	if !cfg.IsReplicationEnabled() {
		return
	}
	if cfg.Env == "production" && !cfg.S3Enabled {
		logger.Error("HA replication requires S3 storage in production (local project storage is not replicated)")
		os.Exit(1)
	}
	if cfg.Env == "production" && cfg.S3Enabled && cfg.S3AccessKey == "" {
		logger.Error("HA replication with S3 enabled requires S3 credentials in production")
		os.Exit(1)
	}
}
