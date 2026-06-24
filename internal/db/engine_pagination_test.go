package db

import (
	"testing"
)

func TestEngine_Pagination(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("items")

	// Insert 20 documents.
	for i := 0; i < 20; i++ {
		doc := map[string]interface{}{
			"name":  "item",
			"index": i,
		}
		if err := e.Insert("items", "", doc); err != nil {
			t.Fatal(err)
		}
	}

	// Count.
	count, err := e.Count("items")
	if err != nil {
		t.Fatal(err)
	}
	if count != 20 {
		t.Fatalf("expected 20, got %d", count)
	}

	// ListPaged default limit (50) with offset 0.
	docs, err := e.ListPaged("items", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 20 {
		t.Fatalf("expected all 20 docs, got %d", len(docs))
	}

	// ListPaged with limit 5, offset 0.
	docs, err = e.ListPaged("items", 5, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 5 {
		t.Fatalf("expected 5 docs, got %d", len(docs))
	}

	// ListPaged with limit 5, offset 10.
	docs, err = e.ListPaged("items", 5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 5 {
		t.Fatalf("expected 5 docs, got %d", len(docs))
	}

	// ListPaged with offset beyond total.
	docs, err = e.ListPaged("items", 5, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected 0 docs, got %d", len(docs))
	}
}

func TestEngine_PaginationLimitCap(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("large")

	for i := 0; i < 1500; i++ {
		if err := e.Insert("large", "", map[string]interface{}{"i": i}); err != nil {
			t.Fatal(err)
		}
	}

	// Limit > 1000 should be capped to 1000.
	docs, err := e.ListPaged("large", 9999, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1000 {
		t.Fatalf("expected 1000 docs (capped), got %d", len(docs))
	}
}

func TestEngine_QueryPaged(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("users")

	for i := 0; i < 100; i++ {
		role := "user"
		if i%2 == 0 {
			role = "admin"
		}
		if err := e.Insert("users", "", map[string]interface{}{
			"name":   "u",
			"index":  i,
			"role":   role,
			"active": true,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// QueryPaged: filter admins, limit 3, offset 5.
	docs, err := e.QueryPaged("users", map[string]interface{}{
		"role": "admin",
	}, []string{"index", "role"}, 3, 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(docs) != 3 {
		t.Fatalf("expected 3 admins, got %d", len(docs))
	}
	for _, doc := range docs {
		if doc["role"] != "admin" {
			t.Fatalf("expected admin role, got %v", doc["role"])
		}
		if _, ok := doc["name"]; ok {
			t.Fatal("projection should have excluded 'name'")
		}
	}
}

func TestEngine_AutoTimestamps(t *testing.T) {
	e, err := NewMemEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	_ = e.CreateCollection("ts")

	// Insert with auto timestamps.
	doc := map[string]interface{}{"name": "test"}
	if err := e.Insert("ts", "", doc); err != nil {
		t.Fatal(err)
	}

	got, err := e.Get("ts", doc["_id"].(string))
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := got["_createdAt"]; !ok {
		t.Fatal("expected _createdAt on inserted doc")
	}
	if _, ok := got["_updatedAt"]; !ok {
		t.Fatal("expected _updatedAt on inserted doc")
	}

	createdAt := got["_createdAt"].(string)
	updatedAt := got["_updatedAt"].(string)
	if updatedAt != createdAt {
		t.Fatal("_updatedAt should equal _createdAt on insert")
	}

	// Update: _updatedAt should change, _createdAt preserved.
	got["name"] = "updated"
	if err := e.Update("ts", doc["_id"].(string), got); err != nil {
		t.Fatal(err)
	}

	got2, err := e.Get("ts", doc["_id"].(string))
	if err != nil {
		t.Fatal(err)
	}

	newCA := got2["_createdAt"].(string)
	newUA := got2["_updatedAt"].(string)

	if newCA != createdAt {
		t.Fatal("_createdAt should be preserved after update")
	}
	if newUA == updatedAt {
		t.Fatal("_updatedAt should change after update")
	}
}
