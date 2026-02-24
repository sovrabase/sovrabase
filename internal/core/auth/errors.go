package auth

import "errors"

var (
	ErrBootstrapAlreadyDone = errors.New("bootstrap already completed")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrBootstrapRequired    = errors.New("bootstrap required")
	ErrInvalidInput         = errors.New("invalid input")
	ErrUserNotFound         = errors.New("user not found")
	ErrForbidden            = errors.New("forbidden")
	ErrRoleNotFound         = errors.New("role not found")
	ErrScopeNotFound        = errors.New("scope not found")
	ErrConflict             = errors.New("conflict")
	ErrRootImmutable        = errors.New("root is immutable")
	ErrRoleHierarchyCycle   = errors.New("role hierarchy cycle")
)
