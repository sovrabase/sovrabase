package requests

// RegisterRequest represents a user registration request
type RegisterRequest struct {
	Username string `json:"username" binding:"required" example:"john_doe"`
	Email    string `json:"email" binding:"required" example:"john@example.com"`
	Password string `json:"password" binding:"required" example:"securepassword123"`
}

// RefreshTokenRequest represents a token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// UpdateUserRequest represents a user update request
type UpdateUserRequest struct {
	Username string `json:"username,omitempty" example:"john_doe"`
	Email    string `json:"email,omitempty" example:"john@example.com"`
	Password string `json:"password,omitempty" example:"newpassword123"`
}

// CreateUserRequest represents admin user creation request
type CreateUserRequest struct {
	Username string   `json:"username" binding:"required" example:"john_doe"`
	Email    string   `json:"email" binding:"required" example:"john@example.com"`
	Password string   `json:"password,omitempty" example:"password123"`
	Roles    []string `json:"roles,omitempty" example:"user,admin"`
}

// UpdateAdminUserRequest represents admin user update request
type UpdateAdminUserRequest struct {
	Username  string   `json:"username,omitempty" example:"john_doe"`
	Email     string   `json:"email,omitempty" example:"john@example.com"`
	Password  string   `json:"password,omitempty" example:"newpassword123"`
	Roles     []string `json:"roles,omitempty" example:"user,admin"`
	CreatedAt string   `json:"created_at,omitempty"`
	UpdatedAt string   `json:"updated_at,omitempty"`
}

// CreateOrganizationRequest represents organization creation request
type CreateOrganizationRequest struct {
	Name string `json:"name" binding:"required" example:"My Organization"`
}

// UpdateOrganizationRequest represents organization update request
type UpdateOrganizationRequest struct {
	Name string `json:"name,omitempty" example:"Updated Organization"`
}

// AddOrganizationMemberRequest represents add member request
type AddOrganizationMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required" example:"member"`
}

// UpdateOrganizationMemberRequest represents update member request
type UpdateOrganizationMemberRequest struct {
	Role string `json:"role" binding:"required" example:"admin"`
}

// CreateOrganizationInvitationRequest represents invitation creation request
type CreateOrganizationInvitationRequest struct {
	Email string `json:"email" binding:"required" example:"user@example.com"`
	Role  string `json:"role" binding:"required" example:"member"`
}

// CreateProjectRequest represents project creation request
type CreateProjectRequest struct {
	Name         string   `json:"name" binding:"required" example:"My Project"`
	OrgID        string   `json:"org_id,omitempty"`
	Capabilities []string `json:"capabilities,omitempty" example:"database,storage,auth"`
}

// UpdateProjectRequest represents project update request
type UpdateProjectRequest struct {
	Name         string                 `json:"name,omitempty" example:"Updated Project"`
	Capabilities map[string]interface{} `json:"capabilities,omitempty"`
	CORS         []string               `json:"cors,omitempty"`
	Members      []ProjectMember        `json:"members,omitempty"`
}

// ProjectMember represents a project member in update request
type ProjectMember struct {
	Username string `json:"username" example:"john_doe"`
	Role     string `json:"role" example:"developer"`
}

// AddProjectMemberRequest represents add project member request
type AddProjectMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required" example:"developer"`
}

// UpdateProjectMemberRequest represents update project member request
type UpdateProjectMemberRequest struct {
	Role string `json:"role" binding:"required" example:"admin"`
}

// CreateAPIKeyRequest represents API key creation request
type CreateAPIKeyRequest struct {
	Name        string   `json:"name" binding:"required" example:"Production Key"`
	Description string   `json:"description" binding:"required" example:"Key for production environment"`
	Permissions []string `json:"permissions,omitempty" example:"read,write"`
	ExpiresAt   string   `json:"expires_at,omitempty" example:"2025-12-31T23:59:59Z"`
}

// UpdateAPIKeyRequest represents API key update request
type UpdateAPIKeyRequest struct {
	Name        string   `json:"name,omitempty" example:"Updated Key"`
	Description string   `json:"description,omitempty" example:"Updated description"`
	Permissions []string `json:"permissions,omitempty" example:"read"`
	ExpiresAt   string   `json:"expires_at,omitempty" example:"2026-12-31T23:59:59Z"`
}

// CreateRoleRequest represents role creation request
type CreateRoleRequest struct {
	Name        string   `json:"name" binding:"required" example:"developer"`
	Permissions []string `json:"permissions" binding:"required" example:"read,write"`
}

// UpdateRoleRequest represents role update request
type UpdateRoleRequest struct {
	Name        string   `json:"name,omitempty" example:"senior_developer"`
	Permissions []string `json:"permissions,omitempty" example:"read,write,delete"`
}

// CreateDatabaseRequest represents database creation request
type CreateDatabaseRequest struct {
	Name   string `json:"name" binding:"required" example:"my_database"`
	Engine string `json:"engine" binding:"required" example:"postgres"`
}

// UpdateDatabaseRequest represents database update request
type UpdateDatabaseRequest struct {
	Name string `json:"name,omitempty" example:"updated_database"`
}

// CreateDatabaseBackupRequest represents backup creation request
type CreateDatabaseBackupRequest struct {
	Description string `json:"description,omitempty" example:"Daily backup"`
}

// RestoreDatabaseRequest represents database restore request
type RestoreDatabaseRequest struct {
	BackupID string `json:"backup_id" binding:"required"`
}

// UpdateCollectionRequest represents collection update request
type UpdateCollectionRequest struct {
	Name string `json:"name,omitempty" example:"updated_collection"`
}

// QueryCollectionRequest represents data query request
type QueryCollectionRequest struct {
	Filter map[string]interface{} `json:"filter,omitempty"`
	Sort   map[string]interface{} `json:"sort,omitempty"`
	Limit  int                    `json:"limit,omitempty" example:"10"`
	Offset int                    `json:"offset,omitempty" example:"0"`
}

// InsertDataRequest represents data insertion request
type InsertDataRequest struct {
	Data interface{} `json:"data" binding:"required"`
}

// UpsertDataRequest represents data upsert request
type UpsertDataRequest struct {
	Data interface{} `json:"data" binding:"required"`
}

// BatchDeleteRequest represents batch delete request
type BatchDeleteRequest struct {
	IDs []string `json:"ids" binding:"required"`
}

// UpdateDocumentRequest represents document update request
type UpdateDocumentRequest struct {
	Data map[string]interface{} `json:"data" binding:"required"`
}

// CreateIndexRequest represents index creation request
type CreateIndexRequest struct {
	Name   string                 `json:"name" binding:"required" example:"idx_email"`
	Fields map[string]interface{} `json:"fields" binding:"required"`
	Unique bool                   `json:"unique,omitempty" example:"true"`
}

// CreateStorageBucketRequest represents storage bucket creation request
type CreateStorageBucketRequest struct {
	Name   string `json:"name" binding:"required" example:"images"`
	Public bool   `json:"public,omitempty" example:"false"`
}

// UploadFileRequest represents file upload request (multipart form)
type UploadFileRequest struct {
	File     []byte            `json:"file" binding:"required"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// UpdateFileMetadataRequest represents file metadata update request
type UpdateFileMetadataRequest struct {
	Metadata map[string]interface{} `json:"metadata" binding:"required"`
}

// BatchDeleteFilesRequest represents batch file delete request
type BatchDeleteFilesRequest struct {
	FileIDs []string `json:"file_ids" binding:"required"`
}

// CreateWebhookRequest represents webhook creation request
type CreateWebhookRequest struct {
	URL    string   `json:"url" binding:"required" example:"https://example.com/webhook"`
	Events []string `json:"events" binding:"required" example:"user.created,user.updated"`
	Active bool     `json:"active,omitempty" example:"true"`
}

// UpdateWebhookRequest represents webhook update request
type UpdateWebhookRequest struct {
	URL    string   `json:"url,omitempty" example:"https://example.com/webhook"`
	Events []string `json:"events,omitempty" example:"user.created"`
	Active bool     `json:"active,omitempty" example:"false"`
}

// ProjectSignupRequest represents project user signup request
type ProjectSignupRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"password123"`
	Username string `json:"username,omitempty" example:"john_doe"`
}

// ProjectLoginRequest represents project user login request
type ProjectLoginRequest struct {
	Email    string `json:"email" binding:"required" example:"user@example.com"`
	Password string `json:"password" binding:"required" example:"password123"`
}

// CreateAuthProviderRequest represents auth provider creation request
type CreateAuthProviderRequest struct {
	Name          string   `json:"name" binding:"required" example:"google"`
	AuthURL       string   `json:"auth_url" binding:"required" example:"https://accounts.google.com/o/oauth2/auth"`
	TokenURL      string   `json:"token_url" binding:"required" example:"https://oauth2.googleapis.com/token"`
	RevocationURL string   `json:"revokation_url" binding:"required" example:"https://oauth2.googleapis.com/revoke"`
	ClientID      string   `json:"client_id" binding:"required"`
	ClientSecret  string   `json:"client_secret" binding:"required"`
	RedirectURIs  []string `json:"redirect_uris" binding:"required" example:"https://myapp.com/callback"`
}

// OAuthCallbackRequest represents OAuth callback request
type OAuthCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state,omitempty"`
}

// CreateFunctionRequest represents function creation request
type CreateFunctionRequest struct {
	Name    string `json:"name" binding:"required" example:"my_function"`
	Runtime string `json:"runtime" binding:"required" example:"nodejs"`
	Code    string `json:"code" binding:"required"`
}

// UpdateFunctionRequest represents function update request
type UpdateFunctionRequest struct {
	Name    string `json:"name,omitempty" example:"updated_function"`
	Runtime string `json:"runtime,omitempty" example:"python"`
	Code    string `json:"code,omitempty"`
}

// InvokeFunctionRequest represents function invocation request
type InvokeFunctionRequest struct {
	Args map[string]interface{} `json:"args,omitempty"`
}

// CreateChannelRequest represents channel creation request
type CreateChannelRequest struct {
	Name string `json:"name" binding:"required" example:"chat_room"`
}

// BroadcastMessageRequest represents broadcast message request
type BroadcastMessageRequest struct {
	Message interface{} `json:"message" binding:"required"`
}

// TrackPresenceRequest represents track presence request
type TrackPresenceRequest struct {
	UserID string                 `json:"user_id" binding:"required"`
	Data   map[string]interface{} `json:"data,omitempty"`
}
