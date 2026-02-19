package store

import (
	"context"
	"errors"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/core/connections"
)

var ErrConnectionNotFound = errors.New("connection not found")

type ConnectionStore interface {
	Migrate(ctx context.Context) error
	Create(ctx context.Context, rec connections.ConnectionRecord) error
	Get(ctx context.Context, projectID, slug string) (connections.ConnectionRecord, error)
	List(ctx context.Context, projectID string) ([]connections.ConnectionRecord, error)
	Delete(ctx context.Context, projectID, slug string) error
	UpdateHealth(ctx context.Context, projectID, slug string, status connections.ConnectionStatus, lastErr *string, checkedAt time.Time) error
}
