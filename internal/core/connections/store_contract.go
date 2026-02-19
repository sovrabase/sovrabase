package connections

import (
	"context"
	"time"
)

type ConnectionStore interface {
	Migrate(ctx context.Context) error
	Create(ctx context.Context, rec ConnectionRecord) error
	Get(ctx context.Context, projectID, slug string) (ConnectionRecord, error)
	List(ctx context.Context, projectID string) ([]ConnectionRecord, error)
	Delete(ctx context.Context, projectID, slug string) error
	UpdateHealth(ctx context.Context, projectID, slug string, status ConnectionStatus, lastErr *string, checkedAt time.Time) error
}
