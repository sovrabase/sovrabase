package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strconv"
	"strings"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func runAdminCreateAdmin(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var email, password string
	fs.StringVar(&email, "name", "", "admin email")
	fs.StringVar(&email, "email", "", "admin email")
	fs.StringVar(&password, "password", "", "admin password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if email == "" || password == "" {
		return fmt.Errorf("--name and --password are required")
	}
	user, err := runtime.authService.CreateAdmin(ctx, actorID, email, password)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, user)
}

func runAdminGetUser(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, userID, err := parseIDCommand(ctx, runtime, "get-user", "id", args)
	if err != nil {
		return err
	}
	user, err := runtime.authService.GetUser(ctx, actorID, userID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, user)
}

func runAdminDeleteUser(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, userID, err := parseIDCommand(ctx, runtime, "delete-user", "id", args)
	if err != nil {
		return err
	}
	if err := runtime.authService.DeleteUser(ctx, actorID, userID); err != nil {
		return err
	}
	return writeJSONOutput(stdout, map[string]any{"status": "deleted", "user_id": userID})
}

func runAdminUpdateUser(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("update-user", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var userID, email, password, role, accountType, active string
	fs.StringVar(&userID, "id", "", "user id")
	fs.StringVar(&email, "name", "", "user email")
	fs.StringVar(&email, "email", "", "user email")
	fs.StringVar(&password, "password", "", "user password")
	fs.StringVar(&role, "role", "", "user role")
	fs.StringVar(&accountType, "account-type", "", "account type")
	fs.StringVar(&active, "active", "", "active status")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if userID == "" {
		return fmt.Errorf("--id is required")
	}

	input := coreauth.UpdateUserInput{ActorUserID: actorID, UserID: userID}
	if email != "" {
		input.Email = &email
	}
	if password != "" {
		input.Password = &password
	}
	if role != "" {
		parsedRole := coreauth.UserRole(strings.TrimSpace(role))
		input.Role = &parsedRole
	}
	if accountType != "" {
		parsedAccountType := coreauth.AccountType(strings.TrimSpace(accountType))
		input.AccountType = &parsedAccountType
	}
	if active != "" {
		value, err := strconv.ParseBool(active)
		if err != nil {
			return fmt.Errorf("invalid --active value: %w", err)
		}
		input.IsActive = &value
	}

	updated, err := runtime.authService.UpdateUser(ctx, input)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, updated)
}

func parseIDCommand(ctx context.Context, runtime cliRuntime, command, flagName string, args []string) (string, string, error) {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return "", "", err
	}
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var id string
	fs.StringVar(&id, flagName, "", flagName)
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if id == "" {
		return "", "", fmt.Errorf("--%s is required", flagName)
	}
	return actorID, id, nil
}
