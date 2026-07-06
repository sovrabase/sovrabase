package main

// @title           Sovrabase API
// @version         1.0
// @description     Sovereign European BaaS — lightweight open-source Supabase alternative. Provides JSON document database (PebbleDB), JWT/OAuth authentication, file storage (local + S3), realtime WebSocket, queues, and config maps — all in a single ~40 MB binary.
// @contact.name    Sovrabase
// @contact.url     https://sovrabase.eu
// @contact.email   hello@sovrabase.eu
// @license.name    AGPL-3.0
// @license.url     https://github.com/sovrabase/sovrabase/blob/main/LICENSE
// @host            localhost:6070
// @BasePath        /
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization
// @description     JWT access token: "Bearer <token>"

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/ketsuna-org/sovrabase/internal/config"
	"github.com/ketsuna-org/sovrabase/internal/functions"
)

func main() {
	if functions.IsFnWorkerProcess(os.Args) {
		if err := functions.WorkerMain(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := config.Load()
	logger.Info("Starting Sovrabase",
		"data_dir", cfg.DataDir,
		"listen_addr", cfg.ListenAddr,
		"role", cfg.Role,
		"node_id", cfg.NodeID,
		"config_file", cfg.ConfigFile,
	)

	runSovrabase(logger, cfg)
}
