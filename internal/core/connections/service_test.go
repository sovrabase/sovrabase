package connections

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ketsuna-org/sovrabase/internal/core/adapters"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
	"github.com/ketsuna-org/sovrabase/internal/core/security"
)

var errNotFound = errors.New("not found")

func TestServiceCreateExternalFailsWhenPingFails(t *testing.T) {
	fakeStore := newMemStore()
	adapter := &fakeAdapter{
		engine:  ConnectionEnginePostgres,
		pingErr: errors.New("db down"),
	}

	service := mustService(t, fakeStore, []adapters.TargetAdapter{adapter}, nil)

	_, err := service.CreateExternal(context.Background(), CreateExternalConnectionRequest{
		ProjectID:   "proj1",
		Slug:        "main-db",
		DisplayName: "Main",
		Engine:      ConnectionEnginePostgres,
		DSN:         "postgres://example",
	})
	if err == nil {
		t.Fatalf("CreateExternal() error = nil, want non-nil")
	}
}

func TestServiceCreateManagedCompensatesOnStoreFailure(t *testing.T) {
	fakeStore := newMemStore()
	fakeStore.createErr = errors.New("insert failed")

	adapter := &fakeAdapter{engine: ConnectionEnginePostgres}
	prov := &fakeProvisioner{
		name: ManagedProviderDocker,
		createResponse: provisioning.ProvisionedDatabase{
			Provider:   string(ManagedProviderDocker),
			ResourceID: "container-1",
			DSN:        "postgres://user:pass@localhost:5432/db?sslmode=disable",
		},
	}

	service := mustService(t, fakeStore, []adapters.TargetAdapter{adapter}, []provisioning.Provisioner{prov})

	_, err := service.CreateManaged(context.Background(), CreateManagedConnectionRequest{
		ProjectID:   "proj1",
		Slug:        "managed-db",
		DisplayName: "Managed DB",
		Engine:      ConnectionEnginePostgres,
		Provider:    ManagedProviderDocker,
	})
	if err == nil {
		t.Fatalf("CreateManaged() error = nil, want non-nil")
	}
	if prov.deleteCalls.Load() != 1 {
		t.Fatalf("DeleteDatabase() calls = %d, want 1 for compensation", prov.deleteCalls.Load())
	}
}

func TestServiceDeleteManagedConnectionDeletesContainerAndRecord(t *testing.T) {
	fakeStore := newMemStore()
	provider := ManagedProviderDocker
	resourceID := "container-123"
	now := time.Now().UTC()
	record := ConnectionRecord{
		ID:                "id-1",
		ProjectID:         "proj1",
		Slug:              "managed-db",
		DisplayName:       "Managed",
		Engine:            ConnectionEngineMongo,
		EncryptedDSN:      "enc:v1:test",
		Managed:           true,
		ManagedProvider:   &provider,
		ManagedResourceID: &resourceID,
		Status:            ConnectionStatusHealthy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	_ = fakeStore.Create(context.Background(), record)

	adapter := &fakeAdapter{engine: ConnectionEngineMongo}
	prov := &fakeProvisioner{name: ManagedProviderDocker}
	service := mustService(t, fakeStore, []adapters.TargetAdapter{adapter}, []provisioning.Provisioner{prov})

	if err := service.Delete(context.Background(), "proj1", "managed-db"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if prov.deleteCalls.Load() != 1 {
		t.Fatalf("DeleteDatabase() calls = %d, want 1", prov.deleteCalls.Load())
	}
	if _, err := fakeStore.Get(context.Background(), "proj1", "managed-db"); err == nil {
		t.Fatalf("connection still exists after Delete()")
	}
}

func mustService(t *testing.T, st ConnectionStore, adaptersList []adapters.TargetAdapter, provisioners []provisioning.Provisioner) Service {
	t.Helper()

	registry, err := adapters.NewRegistry(adaptersList...)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	cipher, err := security.NewDSNCipher([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("NewDSNCipher() error = %v", err)
	}

	runtimeRegistry := NewRegistry(10*time.Minute, 1*time.Minute)
	t.Cleanup(func() {
		_ = runtimeRegistry.Stop(context.Background())
	})

	svc, err := NewService(ServiceDeps{
		Store:        st,
		Adapters:     registry,
		Provisioners: provisioners,
		Registry:     runtimeRegistry,
		Cipher:       cipher,
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return svc
}

type memStore struct {
	mu      sync.Mutex
	records map[string]ConnectionRecord

	createErr error
}

func newMemStore() *memStore {
	return &memStore{records: map[string]ConnectionRecord{}}
}

func (m *memStore) Migrate(context.Context) error { return nil }

func (m *memStore) Create(_ context.Context, rec ConnectionRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.createErr != nil {
		return m.createErr
	}
	key := MappingKey(rec.ProjectID, rec.Slug)
	if _, exists := m.records[key]; exists {
		return errors.New("duplicate")
	}
	if rec.Options == nil {
		rec.Options = map[string]string{}
	}
	m.records[key] = rec
	return nil
}

func (m *memStore) Get(_ context.Context, projectID, slug string) (ConnectionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := MappingKey(projectID, slug)
	rec, ok := m.records[key]
	if !ok {
		return ConnectionRecord{}, errNotFound
	}
	return rec, nil
}

func (m *memStore) List(_ context.Context, projectID string) ([]ConnectionRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []ConnectionRecord
	for _, rec := range m.records {
		if rec.ProjectID == projectID {
			out = append(out, rec)
		}
	}
	return out, nil
}

func (m *memStore) Delete(_ context.Context, projectID, slug string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := MappingKey(projectID, slug)
	if _, ok := m.records[key]; !ok {
		return errNotFound
	}
	delete(m.records, key)
	return nil
}

func (m *memStore) UpdateHealth(_ context.Context, projectID, slug string, status ConnectionStatus, lastErr *string, checkedAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := MappingKey(projectID, slug)
	rec, ok := m.records[key]
	if !ok {
		return errNotFound
	}
	rec.Status = status
	rec.LastError = lastErr
	rec.LastCheckedAt = &checkedAt
	rec.UpdatedAt = checkedAt
	m.records[key] = rec
	return nil
}

type fakeAdapter struct {
	engine ConnectionEngine

	pingErr error
	openErr error

	openCalls atomic.Int32
}

func (f *fakeAdapter) Engine() string { return string(f.engine) }

func (f *fakeAdapter) Ping(_ context.Context, _ string, _ map[string]string) error {
	return f.pingErr
}

func (f *fakeAdapter) Open(_ context.Context, _ string, _ map[string]string) (adapters.RuntimeConnection, error) {
	f.openCalls.Add(1)
	if f.openErr != nil {
		return nil, f.openErr
	}
	return &fakeRuntimeConn{}, nil
}

type fakeProvisioner struct {
	name           ManagedProvider
	createResponse provisioning.ProvisionedDatabase
	createErr      error
	deleteErr      error
	deleteCalls    atomic.Int32
}

func (f *fakeProvisioner) Name() string { return string(f.name) }

func (f *fakeProvisioner) CreateDatabase(_ context.Context, _ provisioning.CreateDatabaseRequest) (provisioning.ProvisionedDatabase, error) {
	if f.createErr != nil {
		return provisioning.ProvisionedDatabase{}, f.createErr
	}
	return f.createResponse, nil
}

func (f *fakeProvisioner) DeleteDatabase(_ context.Context, _ provisioning.DeleteDatabaseRequest) error {
	f.deleteCalls.Add(1)
	return f.deleteErr
}
