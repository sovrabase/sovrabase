package mongo

import (
	"context"
	"fmt"
	"strconv"
	"time"

	coreadapters "github.com/ketsuna-org/sovrabase/internal/core/adapters"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
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
	return string(connections.ConnectionEngineMongo)
}

func (a *Adapter) Ping(ctx context.Context, dsn string, opts map[string]string) error {
	client, err := a.connect(ctx, dsn, opts)
	if err != nil {
		return err
	}
	defer func() {
		_ = client.Disconnect(context.Background())
	}()

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return client.Ping(ctxWithTimeout, readpref.Primary())
}

func (a *Adapter) Open(ctx context.Context, dsn string, opts map[string]string) (coreadapters.RuntimeConnection, error) {
	client, err := a.connect(ctx, dsn, opts)
	if err != nil {
		return nil, err
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	if err := client.Ping(ctxWithTimeout, readpref.Primary()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}

	return &runtimeConnection{
		client:  client,
		timeout: a.timeout,
	}, nil
}

func (a *Adapter) connect(ctx context.Context, dsn string, opts map[string]string) (*mongo.Client, error) {
	clientOpts := options.Client().ApplyURI(dsn)

	if value := opts["max_pool_size"]; value != "" {
		maxPoolSize, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid max_pool_size value %q: %w", value, err)
		}
		clientOpts.SetMaxPoolSize(maxPoolSize)
	}
	if value := opts["min_pool_size"]; value != "" {
		minPoolSize, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid min_pool_size value %q: %w", value, err)
		}
		clientOpts.SetMinPoolSize(minPoolSize)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	client, err := mongo.Connect(ctxWithTimeout, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}
	return client, nil
}

type runtimeConnection struct {
	client  *mongo.Client
	timeout time.Duration
}

func (c *runtimeConnection) Close(ctx context.Context) error {
	if c.client == nil {
		return nil
	}
	closeCtx := ctx
	if closeCtx == nil {
		var cancel context.CancelFunc
		closeCtx, cancel = context.WithTimeout(context.Background(), c.timeout)
		defer cancel()
	}
	return c.client.Disconnect(closeCtx)
}
