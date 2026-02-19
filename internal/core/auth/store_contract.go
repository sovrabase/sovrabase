package auth

import (
	"context"
	"time"
)

type UserStore interface {
	BootstrapRequired(ctx context.Context) (bool, error)
	CreateFirstAdmin(ctx context.Context, email, passwordHash string) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
	TouchLastLogin(ctx context.Context, userID string, at time.Time) error
}
