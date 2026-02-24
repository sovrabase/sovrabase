package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
)

var errUnknownCommand = errors.New("unknown command")

func runCLI(ctx context.Context, args []string, stdout, stderr io.Writer, logger *log.Logger) error {
	if len(args) == 0 {
		writeCLIUsage(stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		writeCLIUsage(stdout)
		return nil
	case "config":
		cfg, err := loadRuntimeConfig(logger)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		runtime, err := newCLIRuntime(ctx, cfg)
		if err != nil {
			return err
		}
		defer runtime.close()
		return runConfigCLI(ctx, runtime, args[1:], stdout)
	case "auth":
		cfg, err := loadRuntimeConfig(logger)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		runtime, err := newCLIRuntime(ctx, cfg)
		if err != nil {
			return err
		}
		defer runtime.close()
		return runAuthCLI(ctx, runtime, args[1:], stdout)
	case "admin":
		cfg, err := loadRuntimeConfig(logger)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		runtime, err := newCLIRuntime(ctx, cfg)
		if err != nil {
			return err
		}
		defer runtime.close()
		return runAdminCLI(ctx, runtime, args[1:], stdout)
	default:
		writeCLIUsage(stderr)
		return fmt.Errorf("%w: %s (run 'sovrabase help' for usage)", errUnknownCommand, args[0])
	}
}

func writeCLIUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage:")
	_, _ = fmt.Fprintln(w, "  sovrabase serve")
	_, _ = fmt.Fprintln(w, "  sovrabase config status")
	_, _ = fmt.Fprintln(w, "  sovrabase auth login --name <email> --password <password>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin bootstrap --name <email> --password <password>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin create-admin --name <email> --password <password>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin create-user --name <email> --password <password> --role <admin|user|service>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin list-users")
	_, _ = fmt.Fprintln(w, "  sovrabase admin get-user --id <user-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin update-user --id <user-id> [--name ...] [--password ...] [--role ...] [--account-type ...] [--active true|false]")
	_, _ = fmt.Fprintln(w, "  sovrabase admin delete-user --id <user-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin assign-role --user-id <user-id> --role-id <role-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin remove-role --user-id <user-id> --role-id <role-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin create-role --name <name> --description <description>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin list-roles")
	_, _ = fmt.Fprintln(w, "  sovrabase admin get-role --id <role-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin update-role --id <role-id> [--name ...] [--description ...] [--parent-role-id ...]")
	_, _ = fmt.Fprintln(w, "  sovrabase admin delete-role --id <role-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin create-scope --key <key> --description <description>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin list-scopes")
	_, _ = fmt.Fprintln(w, "  sovrabase admin get-scope --id <scope-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin update-scope --id <scope-id> [--key ...] [--description ...]")
	_, _ = fmt.Fprintln(w, "  sovrabase admin delete-scope --id <scope-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin assign-scope --role-id <role-id> --scope-id <scope-id>")
	_, _ = fmt.Fprintln(w, "  sovrabase admin remove-scope --role-id <role-id> --scope-id <scope-id>")
}
