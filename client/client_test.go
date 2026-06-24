package client

import (
	"os"
	"testing"
)

// getTestClient returns a Client configured from environment variables.
// Returns nil if the test URL is not set.
func getTestClient(t *testing.T) *Client {
	t.Helper()

	baseURL := os.Getenv("SOVRABASE_TEST_URL")
	projectKey := os.Getenv("SOVRABASE_TEST_PROJECT_KEY")

	if baseURL == "" {
		t.Skip("SOVRABASE_TEST_URL not set, skipping integration test")
	}
	if projectKey == "" {
		t.Skip("SOVRABASE_TEST_PROJECT_KEY not set, skipping integration test")
	}

	return New(baseURL, projectKey)
}

func TestNewClient(t *testing.T) {
	c := New("http://localhost:8080", "test-key-123")

	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "http://localhost:8080" {
		t.Fatalf("expected baseURL %q, got %q", "http://localhost:8080", c.baseURL)
	}
	if c.projectKey != "test-key-123" {
		t.Fatalf("expected projectKey %q, got %q", "test-key-123", c.projectKey)
	}
	if c.httpClient == nil {
		t.Fatal("expected non-nil httpClient")
	}
}

func TestNewClientTrailingSlash(t *testing.T) {
	c := New("http://localhost:8080/", "key")
	if c.baseURL != "http://localhost:8080" {
		t.Fatalf("expected trailing slash removed, got %q", c.baseURL)
	}
}

func TestSetAuth(t *testing.T) {
	c := New("http://localhost:8080", "key")

	access := "access-token-xyz"
	refresh := "refresh-token-xyz"
	c.SetAuth(access, refresh)

	if c.AccessToken() != access {
		t.Fatalf("expected accessToken %q, got %q", access, c.AccessToken())
	}
	if c.RefreshToken() != refresh {
		t.Fatalf("expected refreshToken %q, got %q", refresh, c.RefreshToken())
	}
}

func TestAPIHealth(t *testing.T) {
	c := getTestClient(t)

	var result map[string]interface{}
	err := c.doJSON("GET", "/health", nil, &result)
	if err != nil {
		t.Fatalf("health endpoint failed: %v", err)
	}

	status, ok := result["status"].(string)
	if !ok {
		t.Fatalf("expected status field in health response, got %v", result)
	}
	if status != "ok" {
		t.Fatalf("expected status 'ok', got %q", status)
	}
}

func TestInsertAndGet(t *testing.T) {
	c := getTestClient(t)

	data := Document{
		"name":  "Test Document",
		"value": 42,
	}

	doc, err := c.Insert("test", data)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	id, ok := doc["_id"].(string)
	if !ok || id == "" {
		t.Fatalf("expected document with _id, got %v", doc)
	}

	// Get the document back.
	retrieved, err := c.Get("test", id)
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}

	if name, _ := retrieved["name"].(string); name != "Test Document" {
		t.Fatalf("expected name 'Test Document', got %q", name)
	}

	// Clean up.
	if err := c.Delete("test", id); err != nil {
		t.Logf("cleanup delete failed: %v", err)
	}
}

func TestUpdate(t *testing.T) {
	c := getTestClient(t)

	// Create a document first.
	doc, err := c.Insert("test", Document{"name": "Update Me"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	id := doc["_id"].(string)
	defer c.Delete("test", id)

	// Update it.
	updated, err := c.Update("test", id, Document{"name": "Updated"})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	if name, _ := updated["name"].(string); name != "Updated" {
		t.Fatalf("expected name 'Updated', got %q", name)
	}
}

func TestDelete(t *testing.T) {
	c := getTestClient(t)

	doc, err := c.Insert("test", Document{"name": "Delete Me"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	id := doc["_id"].(string)

	if err := c.Delete("test", id); err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	// Verify it's gone.
	_, err = c.Get("test", id)
	if err == nil {
		t.Fatal("expected error when getting deleted document")
	}
}

func TestList(t *testing.T) {
	c := getTestClient(t)

	// List without pagination.
	resp, err := c.List("test", nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data in list response")
	}

	// List with pagination.
	resp, err = c.List("test", &ListOptions{Limit: 5, Offset: 0})
	if err != nil {
		t.Fatalf("paged list failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data in paged list response")
	}
}

func TestQuery(t *testing.T) {
	c := getTestClient(t)

	resp, err := c.Query("test", Filter{"name": "Test Document"}, nil)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data in query response")
	}
}

func TestSearch(t *testing.T) {
	c := getTestClient(t)

	// First insert a searchable document.
	doc, err := c.Insert("test", Document{"name": "Searchable Item", "description": "A document for search testing"})
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
	id := doc["_id"].(string)
	defer c.Delete("test", id)

	resp, err := c.Search("test", "Searchable", &SearchOptions{Limit: 10})
	if err != nil {
		t.Fatalf("search failed: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data in search response")
	}
	if resp.Count == 0 {
		t.Log("search returned 0 results, may be expected if index not warmed up")
	}
}

func TestBatch(t *testing.T) {
	c := getTestClient(t)

	ops := []BatchOp{
		{Op: "insert", Data: Document{"name": "Batch Item 1"}},
		{Op: "insert", Data: Document{"name": "Batch Item 2"}},
	}

	resp, err := c.Batch("test", ops)
	if err != nil {
		t.Fatalf("batch failed: %v", err)
	}

	if resp.Total != 2 {
		t.Fatalf("expected 2 results, got %d", resp.Total)
	}

	// Clean up inserted documents.
	for _, r := range resp.Results {
		if r.Success && r.ID != "" {
			c.Delete("test", r.ID)
		}
	}
}

func TestAuthFlows(t *testing.T) {
	c := getTestClient(t)

	// Test sign-up.
	email := "testuser@example.com"
	password := "securePassword123"

	resp, err := c.SignUp(email, password)
	if err != nil {
		// User may already exist, that's fine.
		t.Logf("signup returned: %v", err)
	}
	if resp != nil {
		if resp.AccessToken == "" {
			t.Fatal("expected non-empty access token")
		}
	}

	// Test sign-in.
	resp, err = c.SignIn(email, password)
	if err != nil {
		t.Fatalf("signin failed: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}

	// Test refresh.
	oldAccess := c.AccessToken()
	if err := c.Refresh(); err != nil {
		t.Fatalf("refresh failed: %v", err)
	}
	if c.AccessToken() == oldAccess {
		t.Log("access token did not change after refresh (may be expected)")
	}

	// Test GetMe.
	user, err := c.GetMe()
	if err != nil {
		t.Fatalf("getme failed: %v", err)
	}
	if user.Email != email {
		t.Fatalf("expected email %q, got %q", email, user.Email)
	}

	// Test forgot password.
	_, err = c.ForgotPassword(email)
	if err != nil {
		t.Logf("forgot password returned: %v", err)
	}
}

func TestStorageUploadAndDownload(t *testing.T) {
	c := getTestClient(t)

	// Ensure we have auth for storage operations (storage requires auth).
	resp, err := c.SignIn("testuser@example.com", "securePassword123")
	if err != nil {
		t.Skipf("skipping storage test: auth failed: %v", err)
	}
	_ = resp

	// Upload a test file.
	content := []byte("Hello, Sovrabase Storage!")
	info, err := c.UploadFile("default", "test/hello.txt", bytesReader(content), int64(len(content)), "text/plain")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if info.Bucket != "default" {
		t.Fatalf("expected bucket 'default', got %q", info.Bucket)
	}

	// Download it back.
	reader, dlInfo, err := c.DownloadFile("default", "test/hello.txt")
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer reader.Close()

	buf := make([]byte, len(content))
	n, _ := reader.Read(buf)
	if string(buf[:n]) != string(content) {
		t.Fatalf("downloaded content mismatch: got %q", string(buf[:n]))
	}
	_ = dlInfo

	// List files.
	files, err := c.ListFiles("default", "test/")
	if err != nil {
		t.Fatalf("list files failed: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("expected at least one file in bucket")
	}

	// Delete file.
	if err := c.DeleteFile("default", "test/hello.txt"); err != nil {
		t.Fatalf("delete file failed: %v", err)
	}
}

// bytesReader is a helper to create an io.Reader from a byte slice.
func bytesReader(b []byte) *bytesReadSeeker {
	return &bytesReadSeeker{data: b}
}

type bytesReadSeeker struct {
	data   []byte
	offset int
}

func (r *bytesReadSeeker) Read(p []byte) (int, error) {
	if r.offset >= len(r.data) {
		return 0, nil
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

func (r *bytesReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case 0:
		abs = offset
	case 1:
		abs = int64(r.offset) + offset
	case 2:
		abs = int64(len(r.data)) + offset
	}
	if abs < 0 || abs > int64(len(r.data)) {
		return 0, nil
	}
	r.offset = int(abs)
	return abs, nil
}
