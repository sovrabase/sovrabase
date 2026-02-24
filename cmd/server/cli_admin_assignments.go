package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func runAdminAssignRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, userID, roleID, err := parseUserRole(ctx, runtime, "assign-role", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.AssignRoleToUser(ctx, actorID, userID, roleID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "assigned", "user_id": userID, "role_id": roleID})
}

func runAdminRemoveRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, userID, roleID, err := parseUserRole(ctx, runtime, "remove-role", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.RemoveRoleFromUser(ctx, actorID, userID, roleID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "removed", "user_id": userID, "role_id": roleID})
}

func runAdminAssignScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, roleID, scopeID, err := parseRoleScope(ctx, runtime, "assign-scope", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.AssignScopeToRole(ctx, actorID, roleID, scopeID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "assigned", "role_id": roleID, "scope_id": scopeID})
}

func runAdminRemoveScope(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, roleID, scopeID, err := parseRoleScope(ctx, runtime, "remove-scope", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.RemoveScopeFromRole(ctx, actorID, roleID, scopeID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "removed", "role_id": roleID, "scope_id": scopeID})
}

func parseUserRole(ctx context.Context, runtime cliRuntime, command string, args []string) (string, string, string, error) {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return "", "", "", err
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var userID, roleID string
	fs.StringVar(&userID, "user-id", "", "user id")
	fs.StringVar(&roleID, "role-id", "", "role id")
	if err := fs.Parse(args); err != nil {
		return "", "", "", err
	}
	if userID == "" || roleID == "" {
		return "", "", "", fmt.Errorf("--user-id and --role-id are required")
	}
	return actorID, userID, roleID, nil
}

func parseRoleScope(ctx context.Context, runtime cliRuntime, command string, args []string) (string, string, string, error) {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return "", "", "", err
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var roleID, scopeID string
	fs.StringVar(&roleID, "role-id", "", "role id")
	fs.StringVar(&scopeID, "scope-id", "", "scope id")
	if err := fs.Parse(args); err != nil {
		return "", "", "", err
	}
	if roleID == "" || scopeID == "" {
		return "", "", "", fmt.Errorf("--role-id and --scope-id are required")
	}
	return actorID, roleID, scopeID, nil
}
