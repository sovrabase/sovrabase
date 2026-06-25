package plugin

import "time"

// ─── Record Events ─────────────────────────────────────────────────────

// RecordEvent is passed to record hooks (create, update, delete).
type RecordEvent struct {
	Collection string
	Record     map[string]interface{} // the new/updated record (can be modified)
	OldRecord  map[string]interface{} // nil for create, set for update/delete
	Action     string                 // "create", "update", "delete"
	ProjectID  string
	UserID     string
}

// ─── Collection Events ────────────────────────────────────────────────

// CollectionEvent is passed to collection hooks (admin schema changes).
type CollectionEvent struct {
	Name       string                 // collection name
	Action     string                 // "create", "update", "delete"
	Schema     map[string]interface{} // collection schema/rules
	ProjectID  string
}

// ─── Auth Events ──────────────────────────────────────────────────────

// AuthEvent is passed to auth hooks (signup, signin, refresh, oauth).
type AuthEvent struct {
	Action   string // "signup", "signin", "refresh", "oauth"
	Email    string
	UserID   string
	User     map[string]interface{} // user record (nil for refresh)
	Provider string                 // OAuth provider name (for oauth events)
	Metadata map[string]interface{} // extra data
	// Set Abort to true and AbortMessage to reject.
	Abort        bool
	AbortMessage string
}

// ─── Storage Events ───────────────────────────────────────────────────

// StorageEvent is passed to storage hooks (upload, download, delete).
type StorageEvent struct {
	Bucket      string
	Path        string
	ContentType string
	Size        int64
	Action      string // "upload", "download", "delete"
	ProjectID   string
	UserID      string
}

// ─── Realtime Events ──────────────────────────────────────────────────

// RealtimeEvent is passed to realtime hooks before broadcast.
type RealtimeEvent struct {
	Type       string                 // "insert", "update", "delete"
	Collection string
	DocID      string
	Data       map[string]interface{} // can be modified to transform the payload
	ProjectID  string
	Timestamp  time.Time
}

// ─── Email Events ─────────────────────────────────────────────────────

// EmailEvent is passed to email hooks before sending.
type EmailEvent struct {
	To       []string
	Subject  string
	Body     string // HTML body
	TextBody string // plain text alternative
	From     string
	Template string // template name (if using templates)
	Data     map[string]interface{}
	// Meta holds arbitrary metadata set by the caller.
	Meta map[string]interface{}
}

// ─── Log Events ────────────────────────────────────────────────────────

// LogLevel represents the severity of a log entry.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
)

func (l LogLevel) String() string {
	switch l {
	case LogDebug:
		return "DEBUG"
	case LogInfo:
		return "INFO"
	case LogWarn:
		return "WARN"
	case LogError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// LogEvent is passed to log hooks.
type LogEvent struct {
	Level   LogLevel
	Message string
	Fields  map[string]interface{} // structured key=value pairs
	Time    time.Time
}
