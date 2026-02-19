package connections

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ketsuna-org/sovrabase/internal/core/adapters"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
	"github.com/ketsuna-org/sovrabase/internal/core/security"
)

type connectionService struct {
	store        ConnectionStore
	adapters     *adapters.Registry
	provisioners map[ManagedProvider]provisioning.Provisioner
	registry     *Registry
	cipher       *security.DSNCipher
	nowFn        func() time.Time
}

type ServiceDeps struct {
	Store        ConnectionStore
	Adapters     *adapters.Registry
	Provisioners []provisioning.Provisioner
	Registry     *Registry
	Cipher       *security.DSNCipher
}

func NewService(deps ServiceDeps) (Service, error) {
	if deps.Store == nil {
		return nil, errors.New("store is required")
	}
	if deps.Adapters == nil {
		return nil, errors.New("adapters registry is required")
	}
	if deps.Registry == nil {
		return nil, errors.New("runtime registry is required")
	}
	if deps.Cipher == nil {
		return nil, errors.New("dsn cipher is required")
	}

	provisioners := make(map[ManagedProvider]provisioning.Provisioner, len(deps.Provisioners))
	for _, p := range deps.Provisioners {
		if p == nil {
			continue
		}
		name := ManagedProvider(p.Name())
		if err := name.Validate(); err != nil {
			return nil, err
		}
		provisioners[name] = p
	}

	return &connectionService{
		store:        deps.Store,
		adapters:     deps.Adapters,
		provisioners: provisioners,
		registry:     deps.Registry,
		cipher:       deps.Cipher,
		nowFn:        func() time.Time { return time.Now().UTC() },
	}, nil
}

func (s *connectionService) CreateExternal(ctx context.Context, req CreateExternalConnectionRequest) (ConnectionRecord, error) {
	if err := req.Validate(); err != nil {
		return ConnectionRecord{}, err
	}

	adapter, err := s.adapters.Get(string(req.Engine))
	if err != nil {
		return ConnectionRecord{}, err
	}

	if err := adapter.Ping(ctx, req.DSN, req.Options); err != nil {
		return ConnectionRecord{}, fmt.Errorf("ping target database: %w", err)
	}

	encrypted, err := s.cipher.Encrypt(req.DSN)
	if err != nil {
		return ConnectionRecord{}, fmt.Errorf("encrypt dsn: %w", err)
	}

	now := s.nowFn()
	record := ConnectionRecord{
		ID:           uuid.NewString(),
		ProjectID:    req.ProjectID,
		Slug:         req.Slug,
		DisplayName:  req.DisplayName,
		Engine:       req.Engine,
		EncryptedDSN: encrypted,
		Options:      cloneOptions(req.Options),
		Managed:      false,
		Status:       ConnectionStatusHealthy,
		LastCheckedAt: func() *time.Time {
			t := now
			return &t
		}(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(ctx, record); err != nil {
		return ConnectionRecord{}, err
	}
	return record, nil
}

func (s *connectionService) CreateManaged(ctx context.Context, req CreateManagedConnectionRequest) (ConnectionRecord, error) {
	if err := req.Validate(); err != nil {
		return ConnectionRecord{}, err
	}

	provisioner, err := s.getProvisioner(req.Provider)
	if err != nil {
		return ConnectionRecord{}, err
	}

	provisioned, err := provisioner.CreateDatabase(ctx, provisioning.CreateDatabaseRequest{
		ProjectID: req.ProjectID,
		Slug:      req.Slug,
		Engine:    string(req.Engine),
		Options:   cloneOptions(req.Options),
	})
	if err != nil {
		return ConnectionRecord{}, fmt.Errorf("provision managed database: %w", err)
	}

	adapter, err := s.adapters.Get(string(req.Engine))
	if err != nil {
		return ConnectionRecord{}, err
	}
	mergedOptions := mergeOptions(req.Options, provisioned.Options)

	if err := adapter.Ping(ctx, provisioned.DSN, mergedOptions); err != nil {
		cleanupErr := provisioner.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{
			ProjectID:  req.ProjectID,
			Slug:       req.Slug,
			Engine:     string(req.Engine),
			ResourceID: provisioned.ResourceID,
			Options:    mergedOptions,
		})
		if cleanupErr != nil {
			return ConnectionRecord{}, fmt.Errorf("ping provisioned database: %v; cleanup failed: %w", err, cleanupErr)
		}
		return ConnectionRecord{}, fmt.Errorf("ping provisioned database: %w", err)
	}

	encrypted, err := s.cipher.Encrypt(provisioned.DSN)
	if err != nil {
		return ConnectionRecord{}, fmt.Errorf("encrypt dsn: %w", err)
	}

	providerName := provisioned.Provider
	if providerName == "" {
		providerName = string(req.Provider)
	}
	providerEnum := ManagedProvider(providerName)
	if err := providerEnum.Validate(); err != nil {
		return ConnectionRecord{}, err
	}
	resourceID := provisioned.ResourceID

	now := s.nowFn()
	record := ConnectionRecord{
		ID:                uuid.NewString(),
		ProjectID:         req.ProjectID,
		Slug:              req.Slug,
		DisplayName:       req.DisplayName,
		Engine:            req.Engine,
		EncryptedDSN:      encrypted,
		Options:           mergedOptions,
		Managed:           true,
		ManagedProvider:   &providerEnum,
		ManagedResourceID: &resourceID,
		Status:            ConnectionStatusHealthy,
		LastCheckedAt: func() *time.Time {
			t := now
			return &t
		}(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.Create(ctx, record); err != nil {
		cleanupErr := provisioner.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{
			ProjectID:  req.ProjectID,
			Slug:       req.Slug,
			Engine:     string(req.Engine),
			ResourceID: provisioned.ResourceID,
			Options:    mergedOptions,
		})
		if cleanupErr != nil {
			return ConnectionRecord{}, fmt.Errorf("persist managed connection: %v; cleanup failed: %w", err, cleanupErr)
		}
		return ConnectionRecord{}, err
	}

	return record, nil
}

func (s *connectionService) Get(ctx context.Context, projectID, slug string) (ConnectionRecord, error) {
	if err := ValidateProjectID(projectID); err != nil {
		return ConnectionRecord{}, err
	}
	if err := ValidateSlug(slug); err != nil {
		return ConnectionRecord{}, err
	}
	return s.store.Get(ctx, projectID, slug)
}

func (s *connectionService) List(ctx context.Context, projectID string) ([]ConnectionRecord, error) {
	if err := ValidateProjectID(projectID); err != nil {
		return nil, err
	}
	return s.store.List(ctx, projectID)
}

func (s *connectionService) Delete(ctx context.Context, projectID, slug string) error {
	if err := ValidateProjectID(projectID); err != nil {
		return err
	}
	if err := ValidateSlug(slug); err != nil {
		return err
	}

	record, err := s.store.Get(ctx, projectID, slug)
	if err != nil {
		return err
	}

	if record.Managed {
		if record.ManagedProvider == nil || record.ManagedResourceID == nil {
			return errors.New("managed connection missing managed provider or resource id")
		}
		provisioner, err := s.getProvisioner(*record.ManagedProvider)
		if err != nil {
			return err
		}
		if err := provisioner.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{
			ProjectID:  record.ProjectID,
			Slug:       record.Slug,
			Engine:     string(record.Engine),
			ResourceID: *record.ManagedResourceID,
			Options:    cloneOptions(record.Options),
		}); err != nil {
			return fmt.Errorf("delete managed database: %w", err)
		}
	}

	if err := s.store.Delete(ctx, projectID, slug); err != nil {
		return err
	}

	s.registry.Invalidate(MappingKey(projectID, slug))
	return nil
}

func (s *connectionService) Acquire(ctx context.Context, projectID, slug string) (Lease, error) {
	if err := ValidateProjectID(projectID); err != nil {
		return nil, err
	}
	if err := ValidateSlug(slug); err != nil {
		return nil, err
	}

	record, err := s.store.Get(ctx, projectID, slug)
	if err != nil {
		return nil, err
	}

	adapter, err := s.adapters.Get(string(record.Engine))
	if err != nil {
		return nil, err
	}

	dsn, err := s.cipher.Decrypt(record.EncryptedDSN)
	if err != nil {
		return nil, fmt.Errorf("decrypt dsn: %w", err)
	}

	lease, err := s.registry.Acquire(ctx, MappingKey(projectID, slug), func(openCtx context.Context) (RuntimeConnection, error) {
		conn, openErr := adapter.Open(openCtx, dsn, cloneOptions(record.Options))
		if openErr != nil {
			return nil, openErr
		}
		return conn, nil
	})
	if err != nil {
		msg := err.Error()
		_ = s.store.UpdateHealth(ctx, projectID, slug, ConnectionStatusUnreachable, &msg, s.nowFn())
		return nil, err
	}

	_ = s.store.UpdateHealth(ctx, projectID, slug, ConnectionStatusHealthy, nil, s.nowFn())
	return lease, nil
}

func (s *connectionService) getProvisioner(provider ManagedProvider) (provisioning.Provisioner, error) {
	provisioner, ok := s.provisioners[provider]
	if !ok {
		return nil, fmt.Errorf("provisioner %q not found", provider)
	}
	return provisioner, nil
}

func cloneOptions(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

func mergeOptions(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return map[string]string{}
	}
	out := cloneOptions(a)
	for k, v := range b {
		out[k] = v
	}
	return out
}
