// Package main Sovrabase API
//
//	@title			Sovrabase API
//	@version		1.0
//	@description	This is the Sovrabase API server.
//	@host			localhost:9056
//	@BasePath		/
//	@securityDefinitions.apikey	BearerAuth
//	@in		header
//	@name		Authorization
package main

import (
	"context"
	"log"
	"os"
)

func main() {
	logger := log.New(os.Stdout, "[sovrabase] ", log.LstdFlags|log.Lmsgprefix)

	cfg, err := loadRuntimeConfig(logger)
	if err != nil {
		logger.Fatalf("configuration error: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	if err := runServerLifecycle(shutdownCtx, cfg, logger); err != nil {
		logger.Fatalf("server lifecycle failed: %v", err)
	}
}
