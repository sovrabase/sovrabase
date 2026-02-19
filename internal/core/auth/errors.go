package auth

import "errors"

var (
	ErrBootstrapAlreadyDone = errors.New("bootstrap already completed")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrBootstrapRequired    = errors.New("bootstrap required")
	ErrInvalidInput         = errors.New("invalid input")
	ErrUserNotFound         = errors.New("user not found")
)
