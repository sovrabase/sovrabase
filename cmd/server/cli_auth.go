package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func runAuthCLI(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		writeAuthUsage(stdout)
		return fmt.Errorf("auth subcommand is required")
	}

	switch args[0] {
	case "login":
		return runAuthLogin(ctx, runtime, args[1:], stdout)
	default:
		return fmt.Errorf("%w: auth %s", errUnknownCommand, args[0])
	}
}

func runAuthLogin(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var email string
	var password string
	fs.StringVar(&email, "name", "", "admin email")
	fs.StringVar(&email, "email", "", "admin email")
	fs.StringVar(&password, "password", "", "admin password")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if email == "" {
		return fmt.Errorf("--name is required")
	}
	if password == "" {
		return fmt.Errorf("--password is required")
	}

	result, err := runtime.authService.Login(ctx, email, password)
	if err != nil {
		return err
	}
	if err := runtime.stateStore.Save(cliState{ActorUserID: result.User.ID}); err != nil {
		return err
	}

	return writeJSONOutput(stdout, map[string]any{
		"status":        "logged_in",
		"actor_user_id": result.User.ID,
		"email":         result.User.Email,
		"role":          result.User.Role,
	})
}

func writeAuthUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Available auth commands:")
	_, _ = fmt.Fprintln(w, "  login    Login as an existing admin user")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Run 'sovrabase auth <command> --help' for more information.")
}
