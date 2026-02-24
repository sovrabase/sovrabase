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

	_ = loadDotEnvIfExists(".env")

	if err := run(context.Background(), os.Args[1:], logger); err != nil {
		logger.Fatalf("runtime error: %v", err)
	}
}

func run(ctx context.Context, args []string, logger *log.Logger) error {
	if len(args) > 0 && args[0] != "serve" {
		return runCLI(ctx, args, os.Stdout, os.Stderr, logger)
	}

	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	if len(args) > 0 {
		return errUnknownCommand
	}

	cfg, err := loadRuntimeConfig(logger)
	if err != nil {
		return err
	}

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	defer shutdownCancel()

	return runServerLifecycle(shutdownCtx, cfg, logger)
}
