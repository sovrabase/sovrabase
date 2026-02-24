package main

import (
	"context"
	"flag"
	"fmt"
	"io"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func runAdminCreateRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("create-role", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var name, description, parentRoleID string
	fs.StringVar(&name, "name", "", "role name")
	fs.StringVar(&description, "description", "", "role description")
	fs.StringVar(&parentRoleID, "parent-role-id", "", "parent role id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if name == "" || description == "" {
		return fmt.Errorf("--name and --description are required")
	}
	input := coreauth.CreateRoleInput{ActorUserID: actorID, Name: name, Description: description}
	if parentRoleID != "" {
		input.ParentRoleID = &parentRoleID
	}
	created, err := runtime.authService.CreateRole(ctx, input)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, created)
}

func runAdminListRoles(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("list-roles takes no arguments")
	}
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	roles, err := runtime.authService.ListRoles(ctx, actorID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, roles)
}

func runAdminGetRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, roleID, err := parseIDCommand(ctx, runtime, "get-role", "id", args)
	if err != nil {
		return err
	}
	role, err := runtime.authService.GetRole(ctx, actorID, roleID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, role)
}

func runAdminDeleteRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, roleID, err := parseIDCommand(ctx, runtime, "delete-role", "id", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.DeleteRole(ctx, actorID, roleID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "deleted", "role_id": roleID})
}

func runAdminUpdateRole(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("update-role", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var roleID, name, description, parentRoleID string
	fs.StringVar(&roleID, "id", "", "role id")
	fs.StringVar(&name, "name", "", "name")
	fs.StringVar(&description, "description", "", "description")
	fs.StringVar(&parentRoleID, "parent-role-id", "", "parent role id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if roleID == "" {
		return fmt.Errorf("--id is required")
	}
	input := coreauth.UpdateRoleInput{ActorUserID: actorID, RoleID: roleID}
	if name != "" {
		input.Name = &name
	}
	if description != "" {
		input.Description = &description
	}
	if parentRoleID != "" {
		input.ParentRoleID = &parentRoleID
	}
	updated, err := runtime.authService.UpdateRole(ctx, input)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, updated)
}
