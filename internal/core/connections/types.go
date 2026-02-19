package connections

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ConnectionEngine string

const (
	ConnectionEnginePostgres ConnectionEngine = "postgres"
	ConnectionEngineMongo    ConnectionEngine = "mongo"
)

func (e ConnectionEngine) Validate() error {
	switch e {
	case ConnectionEnginePostgres, ConnectionEngineMongo:
		return nil
	default:
		return fmt.Errorf("unsupported engine %q", e)
	}
}

type ManagedProvider string

const (
	ManagedProviderDocker     ManagedProvider = "docker"
	ManagedProviderKubernetes ManagedProvider = "kubernetes"
	ManagedProviderExternal   ManagedProvider = "external"
)

func (p ManagedProvider) Validate() error {
	switch p {
	case ManagedProviderDocker, ManagedProviderKubernetes, ManagedProviderExternal:
		return nil
	default:
		return fmt.Errorf("unsupported managed provider %q", p)
	}
}

type ConnectionStatus string

const (
	ConnectionStatusHealthy     ConnectionStatus = "healthy"
	ConnectionStatusUnreachable ConnectionStatus = "unreachable"
	ConnectionStatusUnknown     ConnectionStatus = "unknown"
)

func (s ConnectionStatus) Validate() error {
	switch s {
	case ConnectionStatusHealthy, ConnectionStatusUnreachable, ConnectionStatusUnknown:
		return nil
	default:
		return fmt.Errorf("unsupported status %q", s)
	}
}

type ConnectionRecord struct {
	ID                string
	ProjectID         string
	Slug              string
	DisplayName       string
	Engine            ConnectionEngine
	EncryptedDSN      string
	Options           map[string]string
	Managed           bool
	ManagedProvider   *ManagedProvider
	ManagedResourceID *string
	Status            ConnectionStatus
	LastError         *string
	LastCheckedAt     *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateExternalConnectionRequest struct {
	ProjectID   string
	Slug        string
	DisplayName string
	Engine      ConnectionEngine
	DSN         string
	Options     map[string]string
}

func (r CreateExternalConnectionRequest) Validate() error {
	if err := ValidateProjectID(r.ProjectID); err != nil {
		return err
	}
	if err := ValidateSlug(r.Slug); err != nil {
		return err
	}
	if err := r.Engine.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(r.DSN) == "" {
		return errors.New("dsn is required")
	}
	return nil
}

type CreateManagedConnectionRequest struct {
	ProjectID   string
	Slug        string
	DisplayName string
	Engine      ConnectionEngine
	Provider    ManagedProvider
	Options     map[string]string
}

func (r CreateManagedConnectionRequest) Validate() error {
	if err := ValidateProjectID(r.ProjectID); err != nil {
		return err
	}
	if err := ValidateSlug(r.Slug); err != nil {
		return err
	}
	if err := r.Engine.Validate(); err != nil {
		return err
	}
	if err := r.Provider.Validate(); err != nil {
		return err
	}
	if r.Provider == ManagedProviderExternal {
		return errors.New("provider external is not allowed for managed connections")
	}
	return nil
}

type Lease interface {
	Connection() RuntimeConnection
	Release()
}

type RuntimeConnection interface {
	Close(ctx context.Context) error
}

type Service interface {
	CreateExternal(ctx context.Context, req CreateExternalConnectionRequest) (ConnectionRecord, error)
	CreateManaged(ctx context.Context, req CreateManagedConnectionRequest) (ConnectionRecord, error)
	Get(ctx context.Context, projectID, slug string) (ConnectionRecord, error)
	List(ctx context.Context, projectID string) ([]ConnectionRecord, error)
	Delete(ctx context.Context, projectID, slug string) error
	Acquire(ctx context.Context, projectID, slug string) (Lease, error)
}

var (
	projectIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{2,64}$`)
	slugPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9-_]{1,62}$`)
)

func ValidateProjectID(projectID string) error {
	if !projectIDPattern.MatchString(projectID) {
		return errors.New("project_id must match [a-zA-Z0-9_-]{2,64}")
	}
	return nil
}

func ValidateSlug(slug string) error {
	if !slugPattern.MatchString(slug) {
		return errors.New("slug must match [a-z0-9][a-z0-9-_]{1,62}")
	}
	return nil
}
