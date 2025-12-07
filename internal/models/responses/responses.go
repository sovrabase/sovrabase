package responses

// User represents a user in API responses
type User struct {
	ID        string   `json:"id"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Roles     []string `json:"roles"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// Organisation represents an organization in API responses
type Organisation struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	OwnerID       string      `json:"owner_id"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	Status        string      `json:"status"`
	MembersCount  int         `json:"members_count"`
	ProjectsCount int         `json:"projects_count"`
	Settings      OrgSettings `json:"settings"`
}

// OrgSettings represents organization settings in API responses
type OrgSettings struct {
	MultiTenant bool               `json:"multi_tenant"`
	Region      string             `json:"region"`
	Compliance  ComplianceSettings `json:"compliance"`
}

// ComplianceSettings represents compliance settings in API responses
type ComplianceSettings struct {
	RGPD  bool `json:"rgpd"`
	HIPAA bool `json:"hipaa"`
}

// Project represents a project in API responses
type Project struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	OrgID        string              `json:"org_id"`
	CreatedAt    string              `json:"created_at"`
	UpdatedAt    string              `json:"updated_at"`
	Status       string              `json:"status"`
	CORS         []string            `json:"cors"`
	Capabilities ProjectCapabilities `json:"capabilities"`
	Members      []ProjectMember     `json:"members"`
}

// ProjectCapabilities represents the capabilities enabled for a project in API responses
type ProjectCapabilities struct {
	Database  DatabaseCapability  `json:"database"`
	Storage   StorageCapability   `json:"storage"`
	Auth      AuthCapability      `json:"auth"`
	Realtime  RealtimeCapability  `json:"realtime"`
	Functions FunctionsCapability `json:"functions"`
	Analytics AnalyticsCapability `json:"analytics"`
}

// DatabaseCapability represents database capabilities in API responses
type DatabaseCapability struct {
	Enabled        bool     `json:"enabled"`
	Engines        []string `json:"engines"`
	MaxConnections int      `json:"max_connections"`
	BackupEnabled  bool     `json:"backup_enabled"`
}

// StorageCapability represents storage capabilities in API responses
type StorageCapability struct {
	Enabled      bool   `json:"enabled"`
	Provider     string `json:"provider"`
	MaxSizeGB    int    `json:"max_size_gb"`
	PublicAccess bool   `json:"public_access"`
}

// AuthCapability represents authentication capabilities in API responses
type AuthCapability struct {
	Enabled        bool           `json:"enabled"`
	Providers      []AuthProvider `json:"providers"`
	MFAEnabled     string         `json:"mfa_enabled"`
	SessionTimeout int            `json:"session_timeout"`
}

// AuthProvider represents an auth provider configuration in API responses
type AuthProvider struct {
	Name          string   `json:"name"`
	AuthURL       string   `json:"auth_url"`
	TokenURL      string   `json:"token_url"`
	RevokationURL string   `json:"revokation_url"`
	ClientID      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	RedirectURIs  []string `json:"redirect_uris"`
}

// RealtimeCapability represents realtime capabilities in API responses
type RealtimeCapability struct {
	Enabled        bool     `json:"enabled"`
	Protocols      []string `json:"protocols"`
	MaxConnections int      `json:"max_connections"`
}

// FunctionsCapability represents functions capabilities in API responses
type FunctionsCapability struct {
	Enabled          bool   `json:"enabled"`
	Runtime          string `json:"runtime"`
	MaxExecutionTime int    `json:"max_execution_time"`
	MemoryLimitMB    int    `json:"memory_limit_mb"`
}

// AnalyticsCapability represents analytics capabilities in API responses
type AnalyticsCapability struct {
	Enabled          bool `json:"enabled"`
	Anonymous        bool `json:"anonymous"`
	MonitorsRequests bool `json:"monitors_requests"`
}

// ProjectMember represents a member of a project in API responses
type ProjectMember struct {
	User User   `json:"user"`
	Role string `json:"role"`
}

// AdminMetric represents a server metric in API responses
type AdminMetric struct {
	Metric      string  `json:"metric"`
	Description string  `json:"description"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	Timestamp   string  `json:"timestamp"`
}

// CreateUserResponse represents the response when creating a user
type CreateUserResponse struct {
	Message string                 `json:"message"`
	Data    CreateUserResponseData `json:"data"`
}

// CreateUserResponseData represents the data in create user response
type CreateUserResponseData struct {
	ID       string `json:"id"`
	Password string `json:"password,omitempty"` // Only set if password was not provided in request
}
