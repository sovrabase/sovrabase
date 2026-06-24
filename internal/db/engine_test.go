package db

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// newTestEngine creates an Engine backed by a temporary directory on disk.
// Pebble does not provide a true in-memory mode when opened with "", so we
// use a temp dir that is cleaned up automatically.
func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	dir, err := os.MkdirTemp("", "sovrabase-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	eng, err := NewEngine(dir)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	t.Cleanup(func() {
		eng.Close()
	})
	return eng
}

// newTestEngineMem creates an Engine in a temporary directory via NewMemEngine.
func newTestEngineMem(t *testing.T) *Engine {
	t.Helper()
	eng, err := NewMemEngine()
	if err != nil {
		t.Fatalf("NewMemEngine: %v", err)
	}
	t.Cleanup(func() {
		eng.Close()
	})
	return eng
}

// =============================================================================
// NewEngine / Close
// =============================================================================

func TestNewEngine(t *testing.T) {
	eng := newTestEngine(t)
	if eng == nil {
		t.Fatal("expected non-nil engine")
	}
	if eng.db == nil {
		t.Fatal("expected non-nil pebble.DB")
	}
}

func TestNewEngineInvalidDir(t *testing.T) {
	// Use a path that is a file, not a directory, to trigger an error.
	tmpDir := t.TempDir()
	fPath := filepath.Join(tmpDir, "notadir")
	if err := os.WriteFile(fPath, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	// Open with a sub-path inside the file (treating file as dir fails).
	eng, err := NewEngine(filepath.Join(fPath, "sub"))
	if err == nil {
		eng.Close()
		t.Fatal("expected error opening pebble on invalid path")
	}
}

func TestClose(t *testing.T) {
	eng := newTestEngine(t)
	if err := eng.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Double close should not panic but may error; accept either.
	_ = eng.Close()
}

// =============================================================================
// CreateCollection / DropCollection
// =============================================================================

func TestCreateCollection(t *testing.T) {
	eng := newTestEngine(t)

	err := eng.CreateCollection("users")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Creating the same collection twice should fail.
	err = eng.CreateCollection("users")
	if err == nil {
		t.Fatal("expected error creating duplicate collection")
	}
}

func TestCreateMultipleCollections(t *testing.T) {
	eng := newTestEngine(t)

	for _, name := range []string{"users", "posts", "comments"} {
		if err := eng.CreateCollection(name); err != nil {
			t.Fatalf("CreateCollection(%q): %v", name, err)
		}
	}
}

func TestDropCollection(t *testing.T) {
	eng := newTestEngine(t)

	eng.CreateCollection("users")

	err := eng.DropCollection("users")
	if err != nil {
		t.Fatalf("DropCollection: %v", err)
	}

	// Dropping again should fail.
	err = eng.DropCollection("users")
	if err == nil {
		t.Fatal("expected error dropping non-existent collection")
	}
}

func TestDropCollectionRemovesDocuments(t *testing.T) {
	eng := newTestEngine(t)

	eng.CreateCollection("items")

	// Insert documents.
	for i := 0; i < 5; i++ {
		eng.Insert("items", "", map[string]interface{}{
			"value": i,
		})
	}

	// Drop collection.
	eng.DropCollection("items")

	// Re-create and verify it's empty.
	eng.CreateCollection("items")
	docs, err := eng.List("items")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}
}

// =============================================================================
// Insert
// =============================================================================

func TestInsert(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	err := eng.Insert("users", "alice", map[string]interface{}{
		"name": "Alice",
		"age":  30,
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	doc, err := eng.Get("users", "alice")
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected non-nil document")
	}
	if doc["_id"] != "alice" {
		t.Fatalf("expected _id=alice, got %v", doc["_id"])
	}
	if doc["name"] != "Alice" {
		t.Fatalf("expected name=Alice, got %v", doc["name"])
	}
	if fmt.Sprint(doc["age"]) != "30" {
		t.Fatalf("expected age=30, got %v", doc["age"])
	}
}

func TestInsertAutoUUID(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("items")

	err := eng.Insert("items", "", map[string]interface{}{
		"title": "auto-generated",
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	docs, err := eng.List("items")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc, got %d", len(docs))
	}
	id, ok := docs[0]["_id"].(string)
	if !ok {
		t.Fatal("expected _id to be a string")
	}
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %d (%q)", len(id), id)
	}
}

func TestInsertIntoMissingCollection(t *testing.T) {
	eng := newTestEngine(t)

	err := eng.Insert("ghosts", "1", map[string]interface{}{
		"boo": true,
	})
	if err == nil {
		t.Fatal("expected error inserting into non-existent collection")
	}
}

func TestInsertDuplicateID(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	eng.Insert("users", "bob", map[string]interface{}{"name": "Bob"})

	// Insert with same id overwrites (Pebble Set is an upsert).
	err := eng.Insert("users", "bob", map[string]interface{}{"name": "Robert"})
	if err != nil {
		t.Fatalf("Insert duplicate: %v", err)
	}

	doc, err := eng.Get("users", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Robert" {
		t.Fatalf("expected overwritten name=Robert, got %v", doc["name"])
	}
}

// =============================================================================
// Get
// =============================================================================

func TestGetNotFound(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	doc, err := eng.Get("users", "nobody")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if doc != nil {
		t.Fatal("expected nil document for missing key")
	}
}

func TestGetWrongCollection(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")
	eng.CreateCollection("posts")
	eng.Insert("users", "1", map[string]interface{}{"name": "X"})

	doc, err := eng.Get("posts", "1")
	if err != nil {
		t.Fatal(err)
	}
	if doc != nil {
		t.Fatal("expected nil document when querying wrong collection")
	}
}

// =============================================================================
// Update
// =============================================================================

func TestUpdate(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")
	eng.Insert("users", "carol", map[string]interface{}{
		"name": "Carol",
	})

	err := eng.Update("users", "carol", map[string]interface{}{
		"name": "Caroline",
		"city": "Paris",
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	doc, err := eng.Get("users", "carol")
	if err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Caroline" {
		t.Fatalf("expected name=Caroline, got %v", doc["name"])
	}
	if doc["city"] != "Paris" {
		t.Fatalf("expected city=Paris, got %v", doc["city"])
	}
	if doc["_id"] != "carol" {
		t.Fatalf("expected _id=carol, got %v", doc["_id"])
	}
}

func TestUpdateMissingDoc(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	err := eng.Update("users", "ghost", map[string]interface{}{"name": "Ghost"})
	if err == nil {
		t.Fatal("expected error updating non-existent document")
	}
}

// =============================================================================
// Delete
// =============================================================================

func TestDelete(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")
	eng.Insert("users", "dave", map[string]interface{}{"name": "Dave"})

	err := eng.Delete("users", "dave")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	doc, err := eng.Get("users", "dave")
	if err != nil {
		t.Fatal(err)
	}
	if doc != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestDeleteMissingDoc(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	err := eng.Delete("users", "ghost")
	if err == nil {
		t.Fatal("expected error deleting non-existent document")
	}
}

// =============================================================================
// List
// =============================================================================

func TestList(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	ids := []string{"a", "b", "c", "d", "e"}
	for _, id := range ids {
		eng.Insert("users", id, map[string]interface{}{
			"seq": id,
		})
	}

	docs, err := eng.List("users")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 5 {
		t.Fatalf("expected 5 docs, got %d", len(docs))
	}

	seen := make(map[string]bool)
	for _, doc := range docs {
		seen[doc["_id"].(string)] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Fatalf("missing doc %q", id)
		}
	}
}

func TestListEmptyCollection(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("empty")

	docs, err := eng.List("empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}
}

func TestListMissingCollection(t *testing.T) {
	eng := newTestEngine(t)
	// Don't create the collection.
	docs, err := eng.List("ghosts")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs for missing collection, got %d", len(docs))
	}
}

// =============================================================================
// Query
// =============================================================================

func TestQueryEquality(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("products")

	type prod struct {
		id       string
		category string
		price    float64
	}
	products := []prod{
		{"p1", "book", 9.99},
		{"p2", "book", 14.99},
		{"p3", "electronics", 299.99},
		{"p4", "book", 19.99},
		{"p5", "electronics", 149.99},
		{"p6", "food", 5.99},
	}
	for _, p := range products {
		eng.Insert("products", p.id, map[string]interface{}{
			"category": p.category,
			"price":    p.price,
		})
	}

	// Query books.
	docs, err := eng.Query("products", map[string]interface{}{
		"category": "book",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Fatalf("expected 3 books, got %d", len(docs))
	}
	for _, doc := range docs {
		if doc["category"] != "book" {
			t.Fatalf("expected category=book, got %v", doc["category"])
		}
	}

	// Query electronics.
	docs, err = eng.Query("products", map[string]interface{}{
		"category": "electronics",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 electronics, got %d", len(docs))
	}
}

func TestQueryMultiField(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	eng.Insert("users", "1", map[string]interface{}{
		"role":   "admin",
		"active": true,
	})
	eng.Insert("users", "2", map[string]interface{}{
		"role":   "admin",
		"active": false,
	})
	eng.Insert("users", "3", map[string]interface{}{
		"role":   "user",
		"active": true,
	})

	docs, err := eng.Query("users", map[string]interface{}{
		"role":   "admin",
		"active": "true", // Query values are matched via fmt.Sprint
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 admin+active, got %d", len(docs))
	}
	if docs[0]["_id"] != "1" {
		t.Fatalf("expected _id=1, got %v", docs[0]["_id"])
	}
}

func TestQueryNoMatch(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("items")

	eng.Insert("items", "x", map[string]interface{}{"color": "red"})

	docs, err := eng.Query("items", map[string]interface{}{
		"color": "blue",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}
}

func TestQueryEmptyFilter(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("items")

	eng.Insert("items", "a", map[string]interface{}{"x": 1})
	eng.Insert("items", "b", map[string]interface{}{"x": 2})

	docs, err := eng.Query("items", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 docs with nil filter, got %d", len(docs))
	}
}

func TestQueryMissingField(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("items")

	eng.Insert("items", "a", map[string]interface{}{"foo": "bar"})

	docs, err := eng.Query("items", map[string]interface{}{
		"baz": "qux",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}
}

func TestQueryAdvancedAndProjections(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("users")

	eng.Insert("users", "1", map[string]interface{}{"name": "Alice", "age": 25, "active": true})
	eng.Insert("users", "2", map[string]interface{}{"name": "Bob", "age": 17, "active": false})
	eng.Insert("users", "3", map[string]interface{}{"name": "Charlie", "age": 30, "active": true})

	// Test inequality $gt operator
	docs, err := eng.Query("users", map[string]interface{}{
		"age": map[string]interface{}{"$gt": 18},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 users aged > 18, got %d", len(docs))
	}

	// Test projection (only select name)
	docs, err = eng.Query("users", map[string]interface{}{
		"active": true,
	}, []string{"name"})
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("expected 2 active users, got %d", len(docs))
	}
	for _, d := range docs {
		if _, ok := d["age"]; ok {
			t.Fatalf("age should be excluded by projection")
		}
		if _, ok := d["name"]; !ok {
			t.Fatalf("name should be included by projection")
		}
		if _, ok := d["_id"]; !ok {
			t.Fatalf("_id should always be included in projection")
		}
	}

	// Test $contains operator
	docs, err = eng.Query("users", map[string]interface{}{
		"name": map[string]interface{}{"$contains": "li"},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 { // Alice and Charlie
		t.Fatalf("expected 2 users containing 'li', got %d", len(docs))
	}
}

// =============================================================================
// Data isolation between collections
// =============================================================================

func TestCollectionIsolation(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("a")
	eng.CreateCollection("b")

	eng.Insert("a", "1", map[string]interface{}{"val": "a1"})
	eng.Insert("a", "2", map[string]interface{}{"val": "a2"})
	eng.Insert("b", "1", map[string]interface{}{"val": "b1"})

	docsA, _ := eng.List("a")
	docsB, _ := eng.List("b")

	if len(docsA) != 2 {
		t.Fatalf("collection a: expected 2, got %d", len(docsA))
	}
	if len(docsB) != 1 {
		t.Fatalf("collection b: expected 1, got %d", len(docsB))
	}
}

// =============================================================================
// Complex document types
// =============================================================================

func TestComplexDocumentTypes(t *testing.T) {
	eng := newTestEngine(t)
	eng.CreateCollection("data")

	eng.Insert("data", "complex", map[string]interface{}{
		"nested": map[string]interface{}{
			"deep": "value",
		},
		"arr":  []interface{}{1.0, 2.0, 3.0},
		"flag": true,
		"num":  42.5,
		"nil":  nil,
	})

	doc, err := eng.Get("data", "complex")
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected document")
	}

	nested, ok := doc["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested map, got %T", doc["nested"])
	}
	if nested["deep"] != "value" {
		t.Fatalf("expected nested.deep=value, got %v", nested["deep"])
	}

	arr, ok := doc["arr"].([]interface{})
	if !ok {
		t.Fatalf("expected arr slice, got %T", doc["arr"])
	}
	if len(arr) != 3 {
		t.Fatalf("expected arr len 3, got %d", len(arr))
	}
}

// =============================================================================
// In-memory engine
// =============================================================================

func TestMemEngine(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("test"); err != nil {
		t.Fatal(err)
	}
	if err := eng.Insert("test", "k", map[string]interface{}{"v": 1}); err != nil {
		t.Fatal(err)
	}

	doc, err := eng.Get("test", "k")
	if err != nil {
		t.Fatal(err)
	}
	if doc == nil {
		t.Fatal("expected doc")
	}
	if fmt.Sprint(doc["v"]) != "1" {
		t.Fatalf("expected v=1, got %v", doc["v"])
	}
}

func TestQueryNewOperators(t *testing.T) {
	eng := newTestEngineMem(t)
	_ = eng.CreateCollection("items")

	_ = eng.Insert("items", "1", map[string]interface{}{"name": "Apple", "type": "fruit", "qty": 10})
	_ = eng.Insert("items", "2", map[string]interface{}{"name": "Banana", "type": "fruit", "qty": 20, "color": "yellow"})
	_ = eng.Insert("items", "3", map[string]interface{}{"name": "Carrot", "type": "vegetable", "qty": 30})

	// 1. Test $startsWith
	docs, err := eng.Query("items", map[string]interface{}{
		"name": map[string]interface{}{"$startsWith": "Ap"},
	}, nil)
	if err != nil || len(docs) != 1 || docs[0]["name"] != "Apple" {
		t.Fatalf("expected Apple, got: %v (err: %v)", docs, err)
	}

	// 2. Test $exists true
	docs, err = eng.Query("items", map[string]interface{}{
		"color": map[string]interface{}{"$exists": true},
	}, nil)
	if err != nil || len(docs) != 1 || docs[0]["name"] != "Banana" {
		t.Fatalf("expected Banana (color exists), got: %v (err: %v)", docs, err)
	}

	// 3. Test $exists false
	docs, err = eng.Query("items", map[string]interface{}{
		"color": map[string]interface{}{"$exists": false},
	}, nil)
	if err != nil || len(docs) != 2 {
		t.Fatalf("expected 2 items without color, got: %v (err: %v)", docs, err)
	}

	// 4. Test $regex
	docs, err = eng.Query("items", map[string]interface{}{
		"name": map[string]interface{}{"$regex": "^B.n.n.$"},
	}, nil)
	if err != nil || len(docs) != 1 || docs[0]["name"] != "Banana" {
		t.Fatalf("expected Banana, got: %v (err: %v)", docs, err)
	}

	// 5. Test $in
	docs, err = eng.Query("items", map[string]interface{}{
		"qty": map[string]interface{}{"$in": []interface{}{10, 30, 99}},
	}, nil)
	if err != nil || len(docs) != 2 {
		t.Fatalf("expected Apple and Carrot (qty 10, 30), got: %v (err: %v)", docs, err)
	}

	// 6. Test $nin
	docs, err = eng.Query("items", map[string]interface{}{
		"qty": map[string]interface{}{"$nin": []interface{}{10, 30}},
	}, nil)
	if err != nil || len(docs) != 1 || docs[0]["name"] != "Banana" {
		t.Fatalf("expected Banana, got: %v (err: %v)", docs, err)
	}
}
