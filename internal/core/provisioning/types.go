package provisioning

import (
	"context"
	"errors"
)

var ErrNotImplemented = errors.New("provisioner not implemented")

type CreateDatabaseRequest struct {
	ProjectID string
	Slug      string
	Engine    string
	Options   map[string]string
}

type ProvisionedDatabase struct {
	Provider   string
	ResourceID string
	DSN        string
	Options    map[string]string
}

type DeleteDatabaseRequest struct {
	ProjectID  string
	Slug       string
	Engine     string
	ResourceID string
	Options    map[string]string
}

type Provisioner interface {
	Name() string
	CreateDatabase(ctx context.Context, req CreateDatabaseRequest) (ProvisionedDatabase, error)
	DeleteDatabase(ctx context.Context, req DeleteDatabaseRequest) error
}
