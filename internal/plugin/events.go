package plugin

// RecordEvent is passed to record hooks (create, update, delete).
// Plugins can read and modify the record, or abort the operation
// by returning an error.
type RecordEvent struct {
	Collection string
	Record     map[string]interface{}
	OldRecord  map[string]interface{} // nil for create, set for update/delete
	Action     string                 // "create", "update", "delete"
	ProjectID  string
	UserID     string
}

// AuthEvent is passed to auth hooks (signup, signin).
type AuthEvent struct {
	Action   string // "signup", "signin"
	Email    string
	UserID   string
	User     map[string]interface{}
	Metadata map[string]interface{} // extra data (provider, etc.)
	// Set Abort to true and AbortMessage to reject the auth.
	Abort        bool
	AbortMessage string
}

// StorageEvent is passed to storage hooks (upload, delete).
type StorageEvent struct {
	Bucket      string
	Path        string
	ContentType string
	Size        int64
	Action      string // "upload", "delete"
	ProjectID   string
	UserID      string
}
