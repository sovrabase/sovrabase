package main

import (
	"context"
	"fmt"
	"io"
)

func runConfigCLI(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		writeConfigUsage(stdout)
		return fmt.Errorf("config subcommand is required")
	}

	switch args[0] {
	case "status":
		return runConfigStatus(ctx, runtime, args[1:], stdout)
	default:
		return fmt.Errorf("%w: config %s", errUnknownCommand, args[0])
	}
}

func runConfigStatus(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("status takes no arguments")
	}
	required, err := runtime.authService.GetConfigState(ctx)
	if err != nil {
		return err
	}

	// Get CLI state directory path
	cliStateDir, err := runtime.stateStore.Path()
	if err != nil {
		return fmt.Errorf("get CLI state path: %w", err)
	}

	return writeJSONOutput(stdout, map[string]any{
		"bootstrap_required": required,
		"metadata_path":      runtime.config.Metadata.SQLite.Path,
		"cli_state_dir":      cliStateDir,
	})
}

func writeConfigUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Available config commands:")
	_, _ = fmt.Fprintln(w, "  status    Check bootstrap status")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Run 'sovrabase config <command> --help' for more information.")
}
