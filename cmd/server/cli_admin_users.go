package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"
)

func runAdminCreateUser(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}

	fs := flag.NewFlagSet("create-user", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var email string
	var password string
	var role string
	var accountType string
	fs.StringVar(&email, "name", "", "user email")
	fs.StringVar(&email, "email", "", "user email")
	fs.StringVar(&password, "password", "", "user password")
	fs.StringVar(&role, "role", "user", "user role")
	fs.StringVar(&accountType, "account-type", "", "account type")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if email == "" {
		return fmt.Errorf("--name is required")
	}
	if password == "" {
		return fmt.Errorf("--password is required")
	}

	userRole, account, err := parseRoleAndAccountType(role, accountType)
	if err != nil {
		return err
	}

	created, err := runtime.authService.CreateUser(ctx, coreauth.CreateUserInput{
		ActorUserID: actorID,
		Email:       email,
		Password:    password,
		Role:        userRole,
		AccountType: account,
	})
	if err != nil {
		return err
	}

	return writeJSONOutput(stdout, created)
}

func runAdminListUsers(ctx context.Context, runtime cliRuntime, args []string, stdout io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("list-users takes no arguments")
	}
	actorID, err := loadActorID(ctx, runtime)
	if err != nil {
		return err
	}
	users, err := runtime.authService.ListUsers(ctx, actorID)
	if err != nil {
		return err
	}
	return writeJSONOutput(stdout, users)
}

func parseRoleAndAccountType(role, accountType string) (coreauth.UserRole, coreauth.AccountType, error) {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		if accountType == "" {
			return coreauth.UserRoleAdmin, coreauth.AccountTypeAdmin, nil
		}
		return coreauth.UserRoleAdmin, coreauth.AccountType(strings.TrimSpace(accountType)), nil
	case "user":
		if accountType == "" {
			return coreauth.UserRoleUser, coreauth.AccountTypeEndUser, nil
		}
		return coreauth.UserRoleUser, coreauth.AccountType(strings.TrimSpace(accountType)), nil
	case "service":
		if accountType == "" {
			return coreauth.UserRoleSvc, coreauth.AccountTypeService, nil
		}
		return coreauth.UserRoleSvc, coreauth.AccountType(strings.TrimSpace(accountType)), nil
	default:
		return "", "", fmt.Errorf("unsupported role %q", role)
	}
}

func writeJSONOutput(w io.Writer, payload any) error {
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
