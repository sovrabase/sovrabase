package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func runAdminBootstrap(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("bootstrap", flag.ContinueOnError)
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

	result, err := runtime.authService.BootstrapFirstAdmin(ctx, email, password)
	if err != nil {
		return err
	}
	if err := runtime.stateStore.Save(cliState{ActorUserID: result.User.ID}); err != nil {
		return err
	}

	return writeJSONOutput(stdout, map[string]any{
		"status":        "bootstrapped",
		"actor_user_id": result.User.ID,
		"email":         result.User.Email,
		"role":          result.User.Role,
	})
}
