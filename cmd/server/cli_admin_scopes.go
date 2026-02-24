package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func runAdminCreateScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("create-scope", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var key, description string
	fs.StringVar(&key, "key", "", "scope key")
	fs.StringVar(&description, "description", "", "scope description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if key == "" || description == "" {
		return fmt.Errorf("--key and --description are required")
	}
	created, err := runtime.authService.CreateScope(ctx, coreauth.CreateScopeInput{
		ActorUserID: actorID,
		Key:         key,
		Description: description,
	})
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, created)
}

func runAdminListScopes(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("list-scopes takes no arguments")
	}
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	scopes, err := runtime.authService.ListScopes(ctx, actorID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, scopes)
}

func runAdminGetScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, scopeID, err := parseIDCommand(ctx, runtime, "get-scope", "id", args)
	if err != nil {
		return err
	}
	scope, err := runtime.authService.GetScope(ctx, actorID, scopeID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, scope)
}

func runAdminDeleteScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, scopeID, err := parseIDCommand(ctx, runtime, "delete-scope", "id", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.DeleteScope(ctx, actorID, scopeID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "deleted", "scope_id": scopeID})
}

func runAdminUpdateScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("update-scope", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var scopeID, key, description string
	fs.StringVar(&scopeID, "id", "", "scope id")
	fs.StringVar(&key, "key", "", "scope key")
	fs.StringVar(&description, "description", "", "scope description")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if scopeID == "" {
		return fmt.Errorf("--id is required")
	}
	input := coreauth.UpdateScopeInput{ActorUserID: actorID, ScopeID: scopeID}
	if key != "" {
		input.Key = &key
	}
	if description != "" {
		input.Description = &description
	}
	updated, err := runtime.authService.UpdateScope(ctx, input)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, updated)
}
