package client

import "time"

// Document is a generic JSON document (map).
type Document map[string]interface{}

// Filter is a generic filter for queries.
type Filter map[string]interface{}

// ListOptions specifies pagination and field selection for List.
type ListOptions struct {
	Limit  int
	Offset int
	Select []string
}

// ListResponse is the paginated response from List and Query.
type ListResponse struct {
	Data   []Document `json:"data"`
	Total  int64      `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// QueryOptions specifies query parameters for the Query method.
type QueryOptions struct {
	Filter     map[string]interface{} `json:"filter,omitempty"`
	Select     []string               `json:"select,omitempty"`
	Limit      int                    `json:"limit,omitempty"`
	Offset     int                    `json:"offset,omitempty"`
}

// SearchOptions specifies search parameters.
type SearchOptions struct {
	Fields []string `json:"fields,omitempty"`
	Limit  int      `json:"limit,omitempty"`
}

// SearchResponse is the response from the Search method.
type SearchResponse struct {
	Data  []Document `json:"data"`
	Count int        `json:"count"`
}

// AuthResponse holds tokens returned from SignIn/SignUp/Refresh.
type AuthResponse struct {
	User         *User  `json:"user,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// User represents an authenticated user.
type User struct {
	ID             string    `json:"id"`
	Email          string    `json:"email"`
	Username       string    `json:"username,omitempty"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
	IsVerified     bool      `json:"is_verified"`
}

// FileInfo holds metadata about a stored file.
type FileInfo struct {
	Bucket      string    `json:"bucket"`
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ContentType string    `json:"content_type"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	URL         string    `json:"url"`
}

// BatchOp represents a single operation in a batch request.
type BatchOp struct {
	Op   string                 `json:"op"`
	ID   string                 `json:"id,omitempty"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// BatchResult is the result of a single operation in a batch response.
type BatchResult struct {
	Index   int         `json:"index"`
	Op      string      `json:"op"`
	ID      string      `json:"id"`
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// BatchResponse is the response from a batch operation.
type BatchResponse struct {
	Results []BatchResult `json:"results"`
	Total   int           `json:"total"`
}

// APIVersion holds the server version info.
type APIVersion struct {
	Version string `json:"version"`
}

// UserInfo represents a user profile returned by UpdateMe / GetMe.
type UserInfo struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name,omitempty"`
	AvatarURL string `json:"avatar_url,omitempty"`
	Role      string `json:"role,omitempty"`
}

// TokenPair holds access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// SignInResult is the response from SignIn, which may require MFA.
// When MFARequired is true, use ChallengeToken with CompleteMFAChallenge.
type SignInResult struct {
	Token          *TokenPair `json:"token,omitempty"`
	MFARequired    bool       `json:"mfa_required"`
	ChallengeToken string     `json:"challenge_token,omitempty"`
	ExpiresIn      int64      `json:"expires_in,omitempty"`
}

// SignedURLResponse holds a pre-signed URL for file access.
type SignedURLResponse struct {
	SignedURL string `json:"signed_url"`
	ExpiresAt string `json:"expires_at"`
}
