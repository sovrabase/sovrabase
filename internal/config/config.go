package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Config holds all configuration for the Sovrabase server.
type Config struct {
	// Core
	DataDir         string        `yaml:"data_dir"       json:"data_dir"`
	ListenAddr      string        `yaml:"listen_addr"    json:"listen_addr"`
	JWTSecret       string        `yaml:"jwt_secret"     json:"jwt_secret"`
	StorageDir      string        `yaml:"storage_dir"    json:"storage_dir"`
	AllowOrigins    string        `yaml:"allow_origins"  json:"allow_origins"`
	AdminEmail      string        `yaml:"admin_email"    json:"admin_email"`
	AdminPassword   string        `yaml:"admin_password" json:"admin_password"`
	SessionDuration time.Duration `yaml:"session_duration" json:"session_duration"`
	BackupInterval  time.Duration `yaml:"backup_interval"  json:"backup_interval"`

	// Security / HTTPS
	CertFile        string        `yaml:"cert_file"        json:"cert_file"`
	KeyFile         string        `yaml:"key_file"         json:"key_file"`
	Env             string        `yaml:"env"              json:"env"`

	// SMTP / Email verification config
	EmailProvider     string `yaml:"email_provider"     json:"email_provider"`     // "smtp", "resend", "mailtrap", "brevo", "mailjet"
	EmailAPIKey       string `yaml:"email_api_key"      json:"email_api_key"`       // API key for HTTP providers
	EmailAPISecret    string `yaml:"email_api_secret"   json:"email_api_secret"`    // API secret (Mailjet)
	SMTPHost          string `yaml:"smtp_host"          json:"smtp_host"`
	SMTPPort          int    `yaml:"smtp_port"          json:"smtp_port"`
	SMTPUser          string `yaml:"smtp_user"          json:"smtp_user"`
	SMTPPassword      string `yaml:"smtp_password"      json:"smtp_password"`
	SMTPSender        string `yaml:"smtp_sender"        json:"smtp_sender"`
	EmailVerification bool   `yaml:"email_verification" json:"email_verification"`

	// Captcha protection
	CaptchaEnabled  bool   `yaml:"captcha_enabled"   json:"captcha_enabled"`
	CaptchaProvider string `yaml:"captcha_provider"  json:"captcha_provider"` // "hcaptcha" or "turnstile"
	CaptchaSiteKey  string `yaml:"captcha_site_key"  json:"captcha_site_key"`
	CaptchaSecret   string `yaml:"captcha_secret"    json:"captcha_secret"`

	// Rate Limiting
	RateLimitPerMinute int `yaml:"rate_limit_per_minute" json:"rate_limit_per_minute"`
	RateLimitBurst     int `yaml:"rate_limit_burst"     json:"rate_limit_burst"`

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

	// Plugins
	PluginsDir string `yaml:"plugins_dir" json:"plugins_dir"`

	// Internal: path to the loaded config file (not saved in YAML)
	ConfigFile string `yaml:"-" json:"-"`
}

// defaults returns a Config with hard-coded fallback values.
func defaults() *Config {
	return &Config{
		DataDir:            filepath.Join(".", "data"),
		ListenAddr:         ":6070",
		JWTSecret:          "change-me-in-production",
		AllowOrigins:       "*",
		AdminEmail:         "admin@sovrabase.eu",
		AdminPassword:      "admin1234",
		SessionDuration:    24 * time.Hour,
		BackupInterval:     1 * time.Hour,
		RateLimitPerMinute: 100,
		RateLimitBurst:     20,
		S3UseSSL:           true,
		S3BucketPrefix:     "sovrabase",
		NodeID:             "node-1",
		ReplAddr:           ":9090",
		LeaseTTL:           5 * time.Second,
		Env:                "development",
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

	// Warn if default JWT secret is used in production environment
	if cfg.JWTSecret == "change-me-in-production" && (strings.ToLower(cfg.Env) == "production" || strings.ToLower(cfg.Env) == "prod") {
		_, _ = fmt.Fprintln(os.Stderr, "WARNING: Using default JWT secret ('change-me-in-production') in PRODUCTION environment! This is a major security risk. Please change it immediately.")
	}

	return cfg
}

// Default is kept for backward compatibility; delegates to Load.
func Default() *Config {
	return Load()
}

// SaveToFile persists the config as YAML to the given path.
// Secrets that equal the sentinel mask "\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022" are NOT written.
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
	setInt := func(env string, dest *int) {
		if v := os.Getenv(env); v != "" {
			if n, err := parseInt(v); err == nil {
				*dest = n
			}
		}
	}

	setStr("SOVRABASE_DATA_DIR", &cfg.DataDir)
	if v := os.Getenv("SOVRABASE_BACKUP_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.BackupInterval = d
		}
	}
	setStr("SOVRABASE_LISTEN_ADDR", &cfg.ListenAddr)
	setStr("SOVRABASE_JWT_SECRET", &cfg.JWTSecret)
	setStr("SOVRABASE_STORAGE_DIR", &cfg.StorageDir)
	setStr("SOVRABASE_ALLOW_ORIGINS", &cfg.AllowOrigins)
	setStr("SOVRABASE_ADMIN_EMAIL", &cfg.AdminEmail)
	setStr("SOVRABASE_ADMIN_PASSWORD", &cfg.AdminPassword)
	setStr("SOVRABASE_ROLE", &cfg.Role)
	setStr("SOVRABASE_NODE_ID", &cfg.NodeID)
	setStr("SOVRABASE_REPL_ADDR", &cfg.ReplAddr)

	setInt("SOVRABASE_RATE_LIMIT_PER_MINUTE", &cfg.RateLimitPerMinute)
	setInt("SOVRABASE_RATE_LIMIT_BURST", &cfg.RateLimitBurst)

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

	setStr("SOVRABASE_CERT_FILE", &cfg.CertFile)
	setStr("SOVRABASE_KEY_FILE", &cfg.KeyFile)
	setStr("SOVRABASE_ENV", &cfg.Env)

	setStr("SOVRABASE_EMAIL_PROVIDER", &cfg.EmailProvider)
	setStr("SOVRABASE_EMAIL_API_KEY", &cfg.EmailAPIKey)
	setStr("SOVRABASE_EMAIL_API_SECRET", &cfg.EmailAPISecret)
	setStr("SOVRABASE_SMTP_HOST", &cfg.SMTPHost)
	setInt("SOVRABASE_SMTP_PORT", &cfg.SMTPPort)
	setStr("SOVRABASE_SMTP_USER", &cfg.SMTPUser)
	setStr("SOVRABASE_SMTP_PASSWORD", &cfg.SMTPPassword)
	setStr("SOVRABASE_SMTP_SENDER", &cfg.SMTPSender)
	setBool("SOVRABASE_EMAIL_VERIFICATION", &cfg.EmailVerification)
	setBool("SOVRABASE_CAPTCHA_ENABLED", &cfg.CaptchaEnabled)
	setStr("SOVRABASE_CAPTCHA_PROVIDER", &cfg.CaptchaProvider)
	setStr("SOVRABASE_CAPTCHA_SITE_KEY", &cfg.CaptchaSiteKey)
	setStr("SOVRABASE_CAPTCHA_SECRET", &cfg.CaptchaSecret)
	setStr("ENV", &cfg.Env)
	setStr("APP_ENV", &cfg.Env)
}

func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
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
