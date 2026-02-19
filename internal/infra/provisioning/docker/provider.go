package docker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/ketsuna-org/sovrabase/internal/core/connections"
	"github.com/ketsuna-org/sovrabase/internal/core/provisioning"
)

const (
	defaultEndpoint      = "unix:///var/run/docker.sock"
	defaultHostAddress   = "127.0.0.1"
	defaultNetworkName   = "sovrabase-managed"
	defaultPostgresImage = "postgres:16-alpine"
	defaultMongoImage    = "mongo:7"
	defaultDockerMode    = "host_port"
)

type Config struct {
	Endpoint      string
	Mode          string
	HostAddress   string
	NetworkName   string
	PostgresImage string
	MongoImage    string
}

type Provider struct {
	cli *client.Client
	cfg Config
}

func NewProvider(cfg Config) (*Provider, error) {
	withDefaults := cfg
	if withDefaults.Endpoint == "" {
		withDefaults.Endpoint = defaultEndpoint
	}
	if withDefaults.Mode == "" {
		withDefaults.Mode = defaultDockerMode
	}
	if withDefaults.HostAddress == "" {
		withDefaults.HostAddress = defaultHostAddress
	}
	if withDefaults.NetworkName == "" {
		withDefaults.NetworkName = defaultNetworkName
	}
	if withDefaults.PostgresImage == "" {
		withDefaults.PostgresImage = defaultPostgresImage
	}
	if withDefaults.MongoImage == "" {
		withDefaults.MongoImage = defaultMongoImage
	}

	if withDefaults.Mode != "host_port" && withDefaults.Mode != "network" {
		return nil, fmt.Errorf("unsupported docker mode %q", withDefaults.Mode)
	}

	cli, err := client.NewClientWithOpts(
		client.WithHost(withDefaults.Endpoint),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &Provider{
		cli: cli,
		cfg: withDefaults,
	}, nil
}

func (p *Provider) Name() string {
	return string(connections.ManagedProviderDocker)
}

func (p *Provider) Close() error {
	return p.cli.Close()
}

func (p *Provider) CreateDatabase(ctx context.Context, req provisioning.CreateDatabaseRequest) (provisioning.ProvisionedDatabase, error) {
	name := containerName(req.ProjectID, req.Slug)

	switch req.Engine {
	case string(connections.ConnectionEnginePostgres):
		return p.createPostgres(ctx, req, name)
	case string(connections.ConnectionEngineMongo):
		return p.createMongo(ctx, req, name)
	default:
		return provisioning.ProvisionedDatabase{}, fmt.Errorf("unsupported engine %q", req.Engine)
	}
}

func (p *Provider) DeleteDatabase(ctx context.Context, req provisioning.DeleteDatabaseRequest) error {
	if strings.TrimSpace(req.ResourceID) == "" {
		return errors.New("resource_id is required")
	}

	stopTimeout := 10
	err := p.cli.ContainerStop(ctx, req.ResourceID, container.StopOptions{Timeout: &stopTimeout})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("stop container %s: %w", req.ResourceID, err)
	}

	err = p.cli.ContainerRemove(ctx, req.ResourceID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("remove container %s: %w", req.ResourceID, err)
	}

	return nil
}

func (p *Provider) createPostgres(ctx context.Context, req provisioning.CreateDatabaseRequest, containerName string) (provisioning.ProvisionedDatabase, error) {
	user, err := randomToken(8)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	password, err := randomToken(24)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	dbName, err := randomToken(10)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}

	user = "sb_" + user
	dbName = "db_" + dbName
	imageRef := pick(req.Options, "image", p.cfg.PostgresImage)

	if err := p.pullImage(ctx, imageRef); err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	if p.cfg.Mode == "network" {
		if err := p.ensureNetwork(ctx); err != nil {
			return provisioning.ProvisionedDatabase{}, err
		}
	}

	internalPort := nat.Port("5432/tcp")
	containerCfg := &container.Config{
		Image: imageRef,
		Env: []string{
			"POSTGRES_USER=" + user,
			"POSTGRES_PASSWORD=" + password,
			"POSTGRES_DB=" + dbName,
		},
		ExposedPorts: nat.PortSet{internalPort: {}},
	}

	hostCfg := &container.HostConfig{}
	if p.cfg.Mode == "host_port" {
		hostCfg.PortBindings = nat.PortMap{
			internalPort: []nat.PortBinding{{HostIP: p.cfg.HostAddress, HostPort: ""}},
		}
	} else {
		hostCfg.NetworkMode = container.NetworkMode(p.cfg.NetworkName)
	}

	resp, err := p.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, fmt.Errorf("create postgres container: %w", err)
	}

	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = p.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{ResourceID: resp.ID})
		return provisioning.ProvisionedDatabase{}, fmt.Errorf("start postgres container: %w", err)
	}

	host, port, err := p.resolveAddress(ctx, resp.ID, containerName, internalPort)
	if err != nil {
		_ = p.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{ResourceID: resp.ID})
		return provisioning.ProvisionedDatabase{}, err
	}

	dsn := postgresDSN(host, port, user, password, dbName)
	return provisioning.ProvisionedDatabase{
		Provider:   string(connections.ManagedProviderDocker),
		ResourceID: resp.ID,
		DSN:        dsn,
		Options: map[string]string{
			"container_name": containerName,
			"host":           host,
			"port":           port,
			"username":       user,
			"password":       password,
			"database":       dbName,
		},
	}, nil
}

func (p *Provider) createMongo(ctx context.Context, req provisioning.CreateDatabaseRequest, containerName string) (provisioning.ProvisionedDatabase, error) {
	user, err := randomToken(8)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	password, err := randomToken(24)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	dbName, err := randomToken(10)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}

	user = "sb_" + user
	dbName = "db_" + dbName
	imageRef := pick(req.Options, "image", p.cfg.MongoImage)

	if err := p.pullImage(ctx, imageRef); err != nil {
		return provisioning.ProvisionedDatabase{}, err
	}
	if p.cfg.Mode == "network" {
		if err := p.ensureNetwork(ctx); err != nil {
			return provisioning.ProvisionedDatabase{}, err
		}
	}

	internalPort := nat.Port("27017/tcp")
	containerCfg := &container.Config{
		Image: imageRef,
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=" + user,
			"MONGO_INITDB_ROOT_PASSWORD=" + password,
			"MONGO_INITDB_DATABASE=" + dbName,
		},
		ExposedPorts: nat.PortSet{internalPort: {}},
	}

	hostCfg := &container.HostConfig{}
	if p.cfg.Mode == "host_port" {
		hostCfg.PortBindings = nat.PortMap{
			internalPort: []nat.PortBinding{{HostIP: p.cfg.HostAddress, HostPort: ""}},
		}
	} else {
		hostCfg.NetworkMode = container.NetworkMode(p.cfg.NetworkName)
	}

	resp, err := p.cli.ContainerCreate(ctx, containerCfg, hostCfg, nil, nil, containerName)
	if err != nil {
		return provisioning.ProvisionedDatabase{}, fmt.Errorf("create mongo container: %w", err)
	}

	if err := p.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		_ = p.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{ResourceID: resp.ID})
		return provisioning.ProvisionedDatabase{}, fmt.Errorf("start mongo container: %w", err)
	}

	host, port, err := p.resolveAddress(ctx, resp.ID, containerName, internalPort)
	if err != nil {
		_ = p.DeleteDatabase(ctx, provisioning.DeleteDatabaseRequest{ResourceID: resp.ID})
		return provisioning.ProvisionedDatabase{}, err
	}

	dsn := mongoDSN(host, port, user, password, dbName)
	return provisioning.ProvisionedDatabase{
		Provider:   string(connections.ManagedProviderDocker),
		ResourceID: resp.ID,
		DSN:        dsn,
		Options: map[string]string{
			"container_name": containerName,
			"host":           host,
			"port":           port,
			"username":       user,
			"password":       password,
			"database":       dbName,
		},
	}, nil
}

func (p *Provider) resolveAddress(ctx context.Context, containerID, containerName string, internalPort nat.Port) (string, string, error) {
	if p.cfg.Mode == "network" {
		return containerName, internalPort.Port(), nil
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		inspect, err := p.cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return "", "", fmt.Errorf("inspect container %s: %w", containerID, err)
		}
		bindings := inspect.NetworkSettings.Ports[internalPort]
		if len(bindings) == 0 {
			time.Sleep(250 * time.Millisecond)
			continue
		}

		host := bindings[0].HostIP
		if host == "" || host == "0.0.0.0" {
			host = p.cfg.HostAddress
		}
		return host, bindings[0].HostPort, nil
	}

	return "", "", errors.New("docker did not allocate a host port before timeout")
}

func (p *Provider) ensureNetwork(ctx context.Context) error {
	_, err := p.cli.NetworkInspect(ctx, p.cfg.NetworkName, network.InspectOptions{})
	if err == nil {
		return nil
	}
	if !errdefs.IsNotFound(err) {
		return fmt.Errorf("inspect network %q: %w", p.cfg.NetworkName, err)
	}

	_, err = p.cli.NetworkCreate(ctx, p.cfg.NetworkName, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil && !isAlreadyExistsErr(err) {
		return fmt.Errorf("create network %q: %w", p.cfg.NetworkName, err)
	}
	return nil
}

func (p *Provider) pullImage(ctx context.Context, imageRef string) error {
	reader, err := p.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image %q: %w", imageRef, err)
	}
	defer reader.Close()

	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func postgresDSN(host, port, user, password, dbName string) string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   "/" + dbName,
	}
	query := u.Query()
	query.Set("sslmode", "disable")
	u.RawQuery = query.Encode()
	return u.String()
}

func mongoDSN(host, port, user, password, dbName string) string {
	u := &url.URL{
		Scheme: "mongodb",
		User:   url.UserPassword(user, password),
		Host:   host + ":" + port,
		Path:   "/" + dbName,
	}
	query := u.Query()
	query.Set("authSource", "admin")
	u.RawQuery = query.Encode()
	return u.String()
}

var invalidNamePattern = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

func containerName(projectID, slug string) string {
	base := fmt.Sprintf("sb-%s-%s-%s", projectID, slug, mustToken(6))
	base = strings.ToLower(base)
	base = invalidNamePattern.ReplaceAllString(base, "-")
	return strings.Trim(base, "-")
}

func mustToken(size int) string {
	token, err := randomToken(size)
	if err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return token
}

func randomToken(size int) (string, error) {
	if size <= 0 {
		return "", errors.New("token size must be > 0")
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return hex.EncodeToString(buf)[:size], nil
}

func isAlreadyExistsErr(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "already exists")
}

func pick(m map[string]string, key, fallback string) string {
	if len(m) == 0 {
		return fallback
	}
	value := strings.TrimSpace(m[key])
	if value == "" {
		return fallback
	}
	return value
}
