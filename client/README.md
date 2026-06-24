# Sovrabase Go SDK

Sovereign European Backend-as-a-Service — GDPR compliant, no US Cloud Act.

## Install

```bash
go get github.com/ketsuna-org/sovrabase/client
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/ketsuna-org/sovrabase/client"
)

func main() {
    db := client.New("https://your-server.sovrabase.eu", "your-project-key")

    // Auth — tokens are stored automatically
    user, err := db.SignUp("user@example.com", "password")
    // Or: pair, err := db.SignIn("user@example.com", "password")

    // CRUD
    doc, _ := db.Insert("users", map[string]interface{}{"name": "Alice", "age": 30})
    id := doc["_id"].(string)

    user, _ := db.Get("users", id)
    db.Update("users", id, map[string]interface{}{"age": 31})
    db.Delete("users", id)

    // List with pagination
    resp, _ := db.List("users", &client.ListOptions{Limit: 20, Offset: 0})
    fmt.Println(resp.Total, len(resp.Data))

    // Search
    results, _ := db.Search("posts", "javascript", &client.SearchOptions{Limit: 10})

    // Batch
    db.Batch("users", []client.BatchOp{
        {Op: "insert", Data: map[string]interface{}{"name": "Bob"}},
        {Op: "update", ID: "abc123", Data: map[string]interface{}{"name": "Updated"}},
    })
}
```

## API

### Auth
```go
// All methods auto-store tokens — no manual SetAuth() needed
db.SignUp(email, password string) (*AuthResponse, error)
db.SignIn(email, password string) (*TokenPair, error)
db.Refresh() error                           // auto-called on 401
db.SetAuth(accessToken, refreshToken string)  // manual override
db.ForgotPassword(email string) (map[string]interface{}, error)
db.ResetPassword(token, newPassword string) error
db.VerifyEmail(token string) error
```

### Database
```go
db.Insert(collection string, data map[string]interface{}) (Document, error)
db.Get(collection, id string) (Document, error)
db.Update(collection, id string, data map[string]interface{}) (Document, error)
db.Delete(collection, id string) error
db.List(collection string, opts *ListOptions) (*ListResponse, error)
db.Query(collection string, filter map[string]interface{}, opts *QueryOptions) (*ListResponse, error)
db.Search(collection, query string, opts *SearchOptions) (*SearchResponse, error)
db.Batch(collection string, ops []BatchOp) (*BatchResponse, error)
```

### Storage
```go
db.UploadFile(bucket, path string, reader io.Reader, size int64, contentType string) (*FileInfo, error)
db.DownloadFile(bucket, path string) (io.ReadCloser, *FileInfo, error)
db.ListFiles(bucket, prefix string) ([]FileInfo, error)
db.DeleteFile(bucket, path string) error
```

### Realtime
```go
rt := db.Realtime()
rt.Connect(ctx)
sub, _ := rt.Subscribe("users", func(event *RealtimeEvent) {
    fmt.Println(event.EventType, event.DocID)
})
defer rt.Close()
```

### Token management
```go
// Custom token persistence across sessions
db.OnTokenRefresh = func(accessToken, refreshToken string) {
    // Save to disk, Keychain, etc.
}
```

## License

MIT
