package postgres

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	coreadapters "github.com/ketsuna-org/sovrabase/internal/core/adapters"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
)

const defaultTimeout = 5 * time.Second

type Adapter struct {
	timeout time.Duration
}

func NewAdapter(timeout time.Duration) *Adapter {
	if timeout <= 0 {
		timeout = defaultTimeout
	}
	return &Adapter{timeout: timeout}
}

func (a *Adapter) Engine() string {
	return string(connections.ConnectionEnginePostgres)
}

func (a *Adapter) Ping(ctx context.Context, dsn string, opts map[string]string) error {
	pool, err := a.newPool(ctx, dsn, opts)
	if err != nil {
		return err
	}
	defer pool.Close()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return pool.Ping(ctxWithTimeout)
}

func (a *Adapter) Open(ctx context.Context, dsn string, opts map[string]string) (coreadapters.RuntimeConnection, error) {
	pool, err := a.newPool(ctx, dsn, opts)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	if err := pool.Ping(ctxWithTimeout); err != nil {
		pool.Close()
		return nil, err
	}

	return &runtimeConnection{pool: pool}, nil
}

func (a *Adapter) newPool(ctx context.Context, dsn string, opts map[string]string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}

	if value := opts["max_conns"]; value != "" {
		maxConns, parseErr := strconv.ParseInt(value, 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid max_conns value %q: %w", value, parseErr)
		}
		cfg.MaxConns = int32(maxConns)
	}
	if value := opts["min_conns"]; value != "" {
		minConns, parseErr := strconv.ParseInt(value, 10, 32)
		if parseErr != nil {
			return nil, fmt.Errorf("invalid min_conns value %q: %w", value, parseErr)
		}
		cfg.MinConns = int32(minConns)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctxWithTimeout, cfg)
	if err != nil {
		return nil, fmt.Errorf("open postgres pool: %w", err)
	}
	return pool, nil
}

type runtimeConnection struct {
	pool *pgxpool.Pool
}

func (c *runtimeConnection) Close(_ context.Context) error {
	if c.pool != nil {
		c.pool.Close()
	}
	return nil
}
