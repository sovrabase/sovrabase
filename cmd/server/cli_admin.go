package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

const bootstrapRequiredMessage = "You should have an admin before performing such commands"

func runAdminCLI(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) == 0 {
		writeAdminUsage(stdout)
		return fmt.Errorf("admin subcommand is required")
	}

	switch args[0] {
	case "bootstrap":
		return runAdminBootstrap(ctx, runtime, args[1:], stdout)
	case "create-admin":
		return runAdminCreateAdmin(ctx, runtime, args[1:], stdout)
	case "create-user":
		return runAdminCreateUser(ctx, runtime, args[1:], stdout)
	case "list-users":
		return runAdminListUsers(ctx, runtime, args[1:], stdout)
	case "get-user":
		return runAdminGetUser(ctx, runtime, args[1:], stdout)
	case "update-user":
		return runAdminUpdateUser(ctx, runtime, args[1:], stdout)
	case "delete-user":
		return runAdminDeleteUser(ctx, runtime, args[1:], stdout)
	case "assign-role":
		return runAdminAssignRole(ctx, runtime, args[1:], stdout)
	case "remove-role":
		return runAdminRemoveRole(ctx, runtime, args[1:], stdout)
	case "create-role":
		return runAdminCreateRole(ctx, runtime, args[1:], stdout)
	case "list-roles":
		return runAdminListRoles(ctx, runtime, args[1:], stdout)
	case "get-role":
		return runAdminGetRole(ctx, runtime, args[1:], stdout)
	case "update-role":
		return runAdminUpdateRole(ctx, runtime, args[1:], stdout)
	case "delete-role":
		return runAdminDeleteRole(ctx, runtime, args[1:], stdout)
	case "create-scope":
		return runAdminCreateScope(ctx, runtime, args[1:], stdout)
	case "list-scopes":
		return runAdminListScopes(ctx, runtime, args[1:], stdout)
	case "get-scope":
		return runAdminGetScope(ctx, runtime, args[1:], stdout)
	case "update-scope":
		return runAdminUpdateScope(ctx, runtime, args[1:], stdout)
	case "delete-scope":
		return runAdminDeleteScope(ctx, runtime, args[1:], stdout)
	case "assign-scope":
		return runAdminAssignScope(ctx, runtime, args[1:], stdout)
	case "remove-scope":
		return runAdminRemoveScope(ctx, runtime, args[1:], stdout)
	default:
		return fmt.Errorf("%w: admin %s", errUnknownCommand, args[0])
	}
}

func loadActorID(ctx context.Context, runtime cliRuntime) (string, error) {
	required, err := runtime.authService.GetConfigState(ctx)
	if err != nil {
		return "", err
	}
	if required {
		return "", errors.New(bootstrapRequiredMessage)
	}

	state, err := runtime.stateStore.Load()
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New(bootstrapRequiredMessage)
		}
		return "", fmt.Errorf("load cli state: %w", err)
	}

	if _, err := runtime.authService.GetUser(ctx, state.ActorUserID, state.ActorUserID); err != nil {
		if errors.Is(err, coreauth.ErrUserNotFound) {
			return "", errors.New(bootstrapRequiredMessage)
		}
		return "", err
	}

	return state.ActorUserID, nil
}

func writeAdminUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Available admin commands:")
	_, _ = fmt.Fprintln(w, "  bootstrap        Bootstrap first admin user")
	_, _ = fmt.Fprintln(w, "  create-admin     Create additional admin user")
	_, _ = fmt.Fprintln(w, "  create-user      Create a new user")
	_, _ = fmt.Fprintln(w, "  list-users       List all users")
	_, _ = fmt.Fprintln(w, "  get-user         Get user by ID")
	_, _ = fmt.Fprintln(w, "  update-user      Update user")
	_, _ = fmt.Fprintln(w, "  delete-user      Delete user")
	_, _ = fmt.Fprintln(w, "  assign-role      Assign role to user")
	_, _ = fmt.Fprintln(w, "  remove-role      Remove role from user")
	_, _ = fmt.Fprintln(w, "  create-role      Create a new role")
	_, _ = fmt.Fprintln(w, "  list-roles       List all roles")
	_, _ = fmt.Fprintln(w, "  get-role         Get role by ID")
	_, _ = fmt.Fprintln(w, "  update-role      Update role")
	_, _ = fmt.Fprintln(w, "  delete-role      Delete role")
	_, _ = fmt.Fprintln(w, "  create-scope     Create a new scope")
	_, _ = fmt.Fprintln(w, "  list-scopes      List all scopes")
	_, _ = fmt.Fprintln(w, "  get-scope        Get scope by ID")
	_, _ = fmt.Fprintln(w, "  update-scope     Update scope")
	_, _ = fmt.Fprintln(w, "  delete-scope     Delete scope")
	_, _ = fmt.Fprintln(w, "  assign-scope     Assign scope to role")
	_, _ = fmt.Fprintln(w, "  remove-scope     Remove scope from role")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "Run 'sovrabase admin <command> --help' for more information on a specific command.")
}
