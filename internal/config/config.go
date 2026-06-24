// Package config handles all Sovrabase server configuration.
//
// Priority (highest to lowest):
//  1. Environment variables
//  2. config.yaml in the data directory
//  3. Hard-coded defaults
package config

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config holds all configuration for the Sovrabase server.
type Config struct {
	// Core
	DataDir       string `yaml:"data_dir"       json:"data_dir"`
	ListenAddr    string `yaml:"listen_addr"    json:"listen_addr"`
	JWTSecret     string `yaml:"jwt_secret"     json:"jwt_secret"`
	StorageDir    string `yaml:"storage_dir"    json:"storage_dir"`
	AllowOrigins  string `yaml:"allow_origins"  json:"allow_origins"`
	AdminEmail    string `yaml:"admin_email"    json:"admin_email"`
	AdminPassword string `yaml:"admin_password" json:"admin_password"`
	SessionDuration time.Duration `yaml:"session_duration" json:"session_duration"` // e.g. 24h, 168h

	// S3 Storage
	S3Enabled      bool   `yaml:"s3_enabled"       json:"s3_enabled"`
	S3Endpoint     string `yaml:"s3_endpoint"      json:"s3_endpoint"`
	S3AccessKey    string `yaml:"s3_access_key"    json:"s3_access_key"`
	S3SecretKey    string `yaml:"s3_secret_key"    json:"s3_secret_key"`
	S3BucketPrefix string `yaml:"s3_bucket_prefix" json:"s3_bucket_prefix"`
	S3UseSSL       bool   `yaml:"s3_use_ssl"       json:"s3_use_ssl"`

	// Replication
	Role     string        `yaml:"role"      json:"role"`      // master, heir, reader, or "" (standalone)
	NodeID   string        `yaml:"node_id"   json:"node_id"`   // unique node identifier
	ReplAddr string        `yaml:"repl_addr" json:"repl_addr"` // WebSocket listen addr for replication
	Peers    []string      `yaml:"peers"     json:"peers"`     // peer addresses
	LeaseTTL time.Duration `yaml:"lease_ttl" json:"lease_ttl"` // lease time-to-live for failover

	// Internal: path to the loaded config file (not saved in YAML)
	ConfigFile string `yaml:"-" json:"-"`
}

// defaults returns a Config with hard-coded fallback values.
func defaults() *Config {
	return &Config{
		DataDir:         filepath.Join(".", "data"),
		ListenAddr:      ":6070",
		JWTSecret:       "change-me-in-production",
		AllowOrigins:    "*",
		AdminEmail:      "admin@sovrabase.eu",
		AdminPassword:   "admin1234",
		SessionDuration: 24 * time.Hour,
		S3UseSSL:        true,
		S3BucketPrefix:  "sovrabase",
		NodeID:          "node-1",
		ReplAddr:        ":9090",
		LeaseTTL:        5 * time.Second,
	}
}

// Load reads configuration using the priority chain:
// env vars > config.yaml > hard-coded defaults.
// It auto-creates a config.yaml if none exists.
func Load() *Config {
	cfg := defaults()

	// 1. Determine dataDir from env first (needed to find the config file)
	if v := os.Getenv("SOVRABASE_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	// 2. Derive config file path and load if it exists
	cfgPath := filepath.Join(cfg.DataDir, "config.yaml")
	cfg.ConfigFile = cfgPath
	if data, err := os.ReadFile(cfgPath); err == nil {
		// Merge file values onto cfg (fields not set in file keep their defaults)
		_ = yaml.Unmarshal(data, cfg)
		cfg.ConfigFile = cfgPath // keep correct path after unmarshal
	}

	// 3. Override with environment variables (highest priority)
	applyEnvOverrides(cfg)

	// 4. Derive StorageDir from DataDir if not explicitly set
	if cfg.StorageDir == "" {
		cfg.StorageDir = filepath.Join(cfg.DataDir, "storage")
	}

	// 5. Push S3 settings into process environment so existing drivers work
	if cfg.S3Enabled && cfg.S3AccessKey != "" {
		setEnvIfEmpty("S3_ACCESS_KEY", cfg.S3AccessKey)
		setEnvIfEmpty("S3_SECRET_KEY", cfg.S3SecretKey)
		setEnvIfEmpty("S3_ENDPOINT", cfg.S3Endpoint)
		setEnvIfEmpty("S3_BUCKET_PREFIX", cfg.S3BucketPrefix)
		if !cfg.S3UseSSL {
			setEnvIfEmpty("S3_USE_SSL", "false")
		}
	}

	// 6. Auto-create config.yaml if missing
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		_ = os.MkdirAll(cfg.DataDir, 0755)
		_ = cfg.SaveToFile(cfgPath)
	}

	return cfg
}

// Default is kept for backward compatibility; delegates to Load.
func Default() *Config {
	return Load()
}

// SaveToFile persists the config as YAML to the given path.
// Secrets that equal the sentinel mask "••••••••" are NOT written.
func (c *Config) SaveToFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// IsReplicationEnabled returns true if the server is configured for replication.
func (c *Config) IsReplicationEnabled() bool {
	return c.Role != "" && c.Role != "standalone"
}

// applyEnvOverrides overlays env variable values onto cfg.
func applyEnvOverrides(cfg *Config) {
	setStr := func(env string, dest *string) {
		if v := os.Getenv(env); v != "" {
			*dest = v
		}
	}
	setBool := func(env string, dest *bool) {
		if v := os.Getenv(env); v != "" {
			*dest = v == "true" || v == "1" || v == "yes"
		}
	}

	setStr("SOVRABASE_DATA_DIR", &cfg.DataDir)
	setStr("SOVRABASE_LISTEN_ADDR", &cfg.ListenAddr)
	setStr("SOVRABASE_JWT_SECRET", &cfg.JWTSecret)
	setStr("SOVRABASE_STORAGE_DIR", &cfg.StorageDir)
	setStr("SOVRABASE_ALLOW_ORIGINS", &cfg.AllowOrigins)
	setStr("SOVRABASE_ADMIN_EMAIL", &cfg.AdminEmail)
	setStr("SOVRABASE_ADMIN_PASSWORD", &cfg.AdminPassword)
	setStr("SOVRABASE_ROLE", &cfg.Role)
	setStr("SOVRABASE_NODE_ID", &cfg.NodeID)
	setStr("SOVRABASE_REPL_ADDR", &cfg.ReplAddr)

	if v := os.Getenv("SOVRABASE_PEERS"); v != "" {
		cfg.Peers = parsePeers(v)
	}

	// S3 env vars (legacy support)
	if v := os.Getenv("S3_ACCESS_KEY"); v != "" {
		cfg.S3Enabled = true
		cfg.S3AccessKey = v
	}
	setStr("S3_SECRET_KEY", &cfg.S3SecretKey)
	setStr("S3_ENDPOINT", &cfg.S3Endpoint)
	setStr("S3_BUCKET_PREFIX", &cfg.S3BucketPrefix)
	setBool("S3_USE_SSL", &cfg.S3UseSSL)
}

func setEnvIfEmpty(key, val string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, val)
	}
}

func parsePeers(raw string) []string {
	var peers []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			peers = append(peers, p)
		}
	}
	return peers
}
