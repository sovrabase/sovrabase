package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath = "config.yaml"
)

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Metadata     MetadataStore      `yaml:"metadata_store"`
	Core         CoreConfig         `yaml:"core"`
	Provisioning ProvisioningConfig `yaml:"provisioning"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type MetadataStore struct {
	Driver   string               `yaml:"driver"`
	SQLite   MetadataSQLiteConfig `yaml:"sqlite"`
	Postgres MetadataPGConfig     `yaml:"postgres"`
}

type MetadataSQLiteConfig struct {
	Path string `yaml:"path"`
}

type MetadataPGConfig struct {
	DSN string `yaml:"dsn"`
}

type CoreConfig struct {
	MasterKeyEnv string `yaml:"master_key_env"`
	CacheTTL     string `yaml:"cache_ttl"`
	Sweep        string `yaml:"sweep_interval"`
}

type ProvisioningConfig struct {
	DefaultProvider string       `yaml:"default_provider"`
	Docker          DockerConfig `yaml:"docker"`
}

type DockerConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Endpoint      string `yaml:"endpoint"`
	Mode          string `yaml:"mode"`
	HostAddress   string `yaml:"host_address"`
	NetworkName   string `yaml:"network_name"`
	PostgresImage string `yaml:"postgres_image"`
	MongoImage    string `yaml:"mongo_image"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Metadata: MetadataStore{
			Driver: "sqlite",
			SQLite: MetadataSQLiteConfig{
				Path: "/data/sovrabase.db",
			},
			Postgres: MetadataPGConfig{
				DSN: "",
			},
		},
		Core: CoreConfig{
			MasterKeyEnv: "SOVRABASE_MASTER_KEY",
			CacheTTL:     "15m",
			Sweep:        "1m",
		},
		Provisioning: ProvisioningConfig{
			DefaultProvider: "docker",
			Docker: DockerConfig{
				Enabled:       true,
				Endpoint:      "unix:///var/run/docker.sock",
				Mode:          "host_port",
				HostAddress:   "127.0.0.1",
				NetworkName:   "sovrabase-managed",
				PostgresImage: "postgres:16-alpine",
				MongoImage:    "mongo:7",
			},
		},
	}
}

func Load() (Config, error) {
	cfg := Default()

	path := os.Getenv("SOVRABASE_CONFIG")
	if path == "" {
		path = defaultConfigPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && path == defaultConfigPath {
			applyEnvOverrides(&cfg)
			return cfg, cfg.Validate()
		}
		return Config{}, fmt.Errorf("read config file %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config file %q: %w", path, err)
	}

	applyEnvOverrides(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) CacheTTLDuration() (time.Duration, error) {
	return time.ParseDuration(c.Core.CacheTTL)
}

func (c Config) SweepDuration() (time.Duration, error) {
	return time.ParseDuration(c.Core.Sweep)
}

func (c Config) Validate() error {
	if c.Server.Host == "" {
		return errors.New("server.host is required")
	}
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}

	switch c.Metadata.Driver {
	case "sqlite":
		if strings.TrimSpace(c.Metadata.SQLite.Path) == "" {
			return errors.New("metadata_store.sqlite.path is required for sqlite driver")
		}
	case "postgres":
		if strings.TrimSpace(c.Metadata.Postgres.DSN) == "" {
			return errors.New("metadata_store.postgres.dsn is required for postgres driver")
		}
	default:
		return fmt.Errorf("unsupported metadata_store.driver %q", c.Metadata.Driver)
	}

	if strings.TrimSpace(c.Core.MasterKeyEnv) == "" {
		return errors.New("core.master_key_env is required")
	}

	cacheTTL, err := c.CacheTTLDuration()
	if err != nil {
		return fmt.Errorf("invalid core.cache_ttl: %w", err)
	}
	if cacheTTL <= 0 {
		return errors.New("core.cache_ttl must be > 0")
	}

	sweep, err := c.SweepDuration()
	if err != nil {
		return fmt.Errorf("invalid core.sweep_interval: %w", err)
	}
	if sweep <= 0 {
		return errors.New("core.sweep_interval must be > 0")
	}

	switch c.Provisioning.DefaultProvider {
	case "docker", "kubernetes", "external":
	default:
		return fmt.Errorf("unsupported provisioning.default_provider %q", c.Provisioning.DefaultProvider)
	}

	switch c.Provisioning.Docker.Mode {
	case "host_port", "network":
	default:
		return fmt.Errorf("unsupported provisioning.docker.mode %q", c.Provisioning.Docker.Mode)
	}

	if c.Provisioning.Docker.Mode == "host_port" && strings.TrimSpace(c.Provisioning.Docker.HostAddress) == "" {
		return errors.New("provisioning.docker.host_address is required when mode is host_port")
	}
	if c.Provisioning.Docker.Mode == "network" && strings.TrimSpace(c.Provisioning.Docker.NetworkName) == "" {
		return errors.New("provisioning.docker.network_name is required when mode is network")
	}
	if strings.TrimSpace(c.Provisioning.Docker.PostgresImage) == "" {
		return errors.New("provisioning.docker.postgres_image is required")
	}
	if strings.TrimSpace(c.Provisioning.Docker.MongoImage) == "" {
		return errors.New("provisioning.docker.mongo_image is required")
	}

	return nil
}

func applyEnvOverrides(cfg *Config) {
	if value := os.Getenv("SOVRABASE_METADATA_DRIVER"); value != "" {
		cfg.Metadata.Driver = value
	}
	if value := os.Getenv("SOVRABASE_METADATA_SQLITE_PATH"); value != "" {
		cfg.Metadata.SQLite.Path = value
	}
	if value := os.Getenv("SOVRABASE_METADATA_POSTGRES_DSN"); value != "" {
		cfg.Metadata.Postgres.DSN = value
	}

	if value := os.Getenv("SOVRABASE_DOCKER_MODE"); value != "" {
		cfg.Provisioning.Docker.Mode = value
	}
	if value := os.Getenv("SOVRABASE_DOCKER_HOST_ADDRESS"); value != "" {
		cfg.Provisioning.Docker.HostAddress = value
	}
	if value := os.Getenv("SOVRABASE_DOCKER_NETWORK_NAME"); value != "" {
		cfg.Provisioning.Docker.NetworkName = value
	}

	if value := os.Getenv("SOVRABASE_SERVER_HOST"); value != "" {
		cfg.Server.Host = value
	}
	if value := os.Getenv("SOVRABASE_SERVER_PORT"); value != "" {
		if port, err := strconv.Atoi(value); err == nil {
			cfg.Server.Port = port
		}
	}
}
