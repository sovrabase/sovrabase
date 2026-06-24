package db

import (
	"testing"
)

func TestEngine_CreateIndex(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("users")

	// Create simple index on "email".
	if err := e.CreateIndex("users", "email", IndexSimple); err != nil {
		t.Fatal(err)
	}

	// Duplicate index should fail.
	if err := e.CreateIndex("users", "email", IndexSimple); err == nil {
		t.Fatal("expected error on duplicate index")
	}

	// List indexes.
	idxs, err := e.ListIndexes("users")
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 1 || idxs[0].Field != "email" || idxs[0].Type != IndexSimple {
		t.Fatalf("unexpected indexes: %+v", idxs)
	}
}

func TestEngine_IndexQueryOptimization(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("items")

	// Insert 100 docs with different "category" values.
	for i := 0; i < 100; i++ {
		cat := "a"
		if i >= 50 {
			cat = "b"
		}
		if err := e.Insert("items", "", map[string]interface{}{
			"name":     "item",
			"category": cat,
			"index":    i,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Query without index: full scan.
	before, _ := e.ListIndexes("items")
	if len(before) != 0 {
		t.Fatal("expected no indexes initially")
	}

	// Create index on "category".
	if err := e.CreateIndex("items", "category", IndexSimple); err != nil {
		t.Fatal(err)
	}

	idxs, err := e.ListIndexes("items")
	if err != nil {
		t.Fatal(err)
	}
	if len(idxs) != 1 || idxs[0].Field != "category" {
		t.Fatalf("expected 1 index, got %+v", idxs)
	}

	// QueryPaged with filter on "category" — should use index.
	docs, err := e.QueryPaged("items", map[string]interface{}{"category": "a"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 10 {
		t.Fatalf("expected 10 category=a docs, got %d", len(docs))
	}
	for _, d := range docs {
		if d["category"] != "a" {
			t.Fatalf("expected category=a, got %v", d["category"])
		}
	}

	// Query all category=a docs (no limit).
	docs, err = e.QueryPaged("items", map[string]interface{}{"category": "a"}, nil, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 50 {
		t.Fatalf("expected 50 category=a docs, got %d", len(docs))
	}
}

func TestEngine_UniqueIndex(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("accounts")

	// Insert a user with unique email "a@b.com".
	if err := e.Insert("accounts", "", map[string]interface{}{
		"email": "a@b.com",
		"name":  "Alice",
	}); err != nil {
		t.Fatal(err)
	}

	// Create unique index — should succeed because there are no duplicates.
	if err := e.CreateIndex("accounts", "email", IndexUnique); err != nil {
		t.Fatal(err)
	}

	// Insert another user with different email — should succeed.
	if err := e.Insert("accounts", "", map[string]interface{}{
		"email": "b@b.com",
		"name":  "Bob",
	}); err != nil {
		t.Fatal(err)
	}

	// Insert user with duplicate email — should fail.
	if err := e.Insert("accounts", "", map[string]interface{}{
		"email": "a@b.com",
		"name":  "Charlie",
	}); err == nil {
		t.Fatal("expected unique constraint violation")
	}

	// Update Bob's email to a@b.com — should fail.
	bobDoc, err := e.Query("accounts", map[string]interface{}{"email": "b@b.com"}, nil)
	if err != nil || len(bobDoc) == 0 {
		t.Fatal("bob not found")
	}
	bobDoc[0]["email"] = "a@b.com"
	if err := e.Update("accounts", bobDoc[0]["_id"].(string), bobDoc[0]); err == nil {
		t.Fatal("expected unique constraint violation on update")
	}
}

func TestEngine_IndexAfterUpdate(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("test")

	_ = e.Insert("test", "", map[string]interface{}{"label": "old"})
	_ = e.CreateIndex("test", "label", IndexSimple)

	// Find via index.
	docs, err := e.QueryPaged("test", map[string]interface{}{"label": "old"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc with label=old, got %d", len(docs))
	}
	docID := docs[0]["_id"].(string)

	// Update label.
	docs[0]["label"] = "new"
	if err := e.Update("test", docID, docs[0]); err != nil {
		t.Fatal(err)
	}

	// Old index entry should be gone.
	docs, err = e.QueryPaged("test", map[string]interface{}{"label": "old"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs with label=old after update, got %d", len(docs))
	}

	// New index entry should exist.
	docs, err = e.QueryPaged("test", map[string]interface{}{"label": "new"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc with label=new, got %d", len(docs))
	}
}

func TestEngine_IndexAfterDelete(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("cache")
	_ = e.Insert("cache", "", map[string]interface{}{"key": "xyz", "val": 42})
	_ = e.CreateIndex("cache", "key", IndexSimple)

	// Confirm index works.
	docs, err := e.QueryPaged("cache", map[string]interface{}{"key": "xyz"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 doc before delete, got %d", len(docs))
	}

	// Delete the document.
	if err := e.Delete("cache", docs[0]["_id"].(string)); err != nil {
		t.Fatal(err)
	}

	// Index entry should be gone.
	docs, err = e.QueryPaged("cache", map[string]interface{}{"key": "xyz"}, nil, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs after delete, got %d", len(docs))
	}
}

func TestEngine_DropIndex(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("stuff")
	_ = e.Insert("stuff", "", map[string]interface{}{"color": "red"})
	_ = e.CreateIndex("stuff", "color", IndexSimple)

	idxs, _ := e.ListIndexes("stuff")
	if len(idxs) != 1 {
		t.Fatal("expected 1 index before drop")
	}

	if err := e.DropIndex("stuff", "color"); err != nil {
		t.Fatal(err)
	}

	idxs, _ = e.ListIndexes("stuff")
	if len(idxs) != 0 {
		t.Fatalf("expected 0 indexes after drop, got %d", len(idxs))
	}
}
