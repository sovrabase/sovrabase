package kubernetes

import (
	"context"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
)

type Provider struct{}

func NewProvider() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "kubernetes"
}

func (p *Provider) CreateDatabase(_ context.Context, _ provisioning.CreateDatabaseRequest) (provisioning.ProvisionedDatabase, error) {
	return provisioning.ProvisionedDatabase{}, provisioning.ErrNotImplemented
}

func (p *Provider) DeleteDatabase(_ context.Context, _ provisioning.DeleteDatabaseRequest) error {
	return provisioning.ErrNotImplemented
}
