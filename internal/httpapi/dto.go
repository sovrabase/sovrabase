package httpapi

import coreauth "github.com/ketsuna-org/sovrabase/internal/core/auth"

type bootstrapRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type apiError struct {
	Error string `json:"error"`
}

type configResponse struct {
	BootstrapRequired bool              `json:"bootstrap_required"`
	Message           string            `json:"message"`
	Auth              configAuthSection `json:"auth"`
	Config            sanitizedConfig   `json:"config"`
}

type configAuthSection struct {
	BootstrapEndpoint string `json:"bootstrap_endpoint"`
	LoginEndpoint     string `json:"login_endpoint"`
	Mode              string `json:"mode"`
}

type sanitizedConfig struct {
	Server       sanitizedServer       `json:"server"`
	Metadata     sanitizedMetadata     `json:"metadata_store"`
	Core         sanitizedCore         `json:"core"`
	Auth         sanitizedAuth         `json:"auth"`
	Provisioning sanitizedProvisioning `json:"provisioning"`
}

type sanitizedServer struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type sanitizedMetadata struct {
	Driver             string `json:"driver"`
	SQLiteConfigured   bool   `json:"sqlite_configured"`
	PostgresConfigured bool   `json:"postgres_configured"`
}

type sanitizedCore struct {
	CacheTTL                string `json:"cache_ttl"`
	SweepInterval           string `json:"sweep_interval"`
	EncryptionKeyConfigured bool   `json:"encryption_key_configured"`
}

type sanitizedAuth struct {
	JWTSigningKeyConfigured bool `json:"jwt_signing_key_configured"`
}

type sanitizedProvisioning struct {
	DefaultProvider string          `json:"default_provider"`
	Docker          sanitizedDocker `json:"docker"`
}

type sanitizedDocker struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode"`
	PostgresImage string `json:"postgres_image"`
	MongoImage    string `json:"mongo_image"`
	Endpoint      string `json:"endpoint"`
	HostAddress   string `json:"host_address"`
	NetworkName   string `json:"network_name"`
}

type bootstrapSuccessResponse struct {
	Message     string `json:"message"`
	TokenType   string `json:"token_type"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	User        struct {
		ID    string            `json:"id"`
		Email string            `json:"email"`
		Role  coreauth.UserRole `json:"role"`
	} `json:"user"`
}
