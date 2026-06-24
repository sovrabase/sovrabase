package db

import (
	"sort"
	"testing"
)

// =============================================================================
// Search
// =============================================================================

func TestSearchInsertingAndSearching(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("docs"); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	docs := []map[string]interface{}{
		{
			"title":   "Go Programming Language",
			"content": "Go is a statically typed compiled programming language designed at Google.",
			"author":  "Robert Griesemer",
		},
		{
			"title":   "Rust Systems Programming",
			"content": "Rust is a multi-paradigm compiled programming language designed for performance and safety.",
			"author":  "Graydon Hoare",
		},
		{
			"title":   "Go Concurrency Patterns",
			"content": "Go has goroutines and channels for concurrent programming. Goroutines are lightweight.",
			"author":  "Rob Pike",
		},
		{
			"title":   "Python for Data Science",
			"content": "Python is a high-level interpreted programming language with dynamic semantics.",
			"author":  "Guido van Rossum",
		},
	}

	for i, doc := range docs {
		// Use a fixed ID for deterministic ordering in assertions.
		id := ""
		if i == 0 {
			id = "doc1"
		} else if i == 1 {
			id = "doc2"
		} else if i == 2 {
			id = "doc3"
		} else {
			id = "doc4"
		}
		if err := eng.Insert("docs", id, doc); err != nil {
			t.Fatalf("Insert doc %d: %v", i, err)
		}
	}

	t.Run("search by single keyword", func(t *testing.T) {
		results, err := eng.Search("docs", "Go", nil, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected at least 1 result, got 0")
		}
		// "Go" appears in doc3 title+content (3 times: title "Go", content "Go" x2),
		// in doc1 title+content (2 times: title "Go", content "Go"),
		// and in doc4 content (0: "Python"). So doc3 should be first.
		if results[0]["_id"] != "doc3" && results[0]["_id"] != "doc1" {
			t.Fatalf("expected doc3 (score 3) or doc1 (score 2) first, got id=%v score=%v",
				results[0]["_id"], results[0]["_search_score"])
		}
		// All results should have _search_score.
		for _, r := range results {
			if _, ok := r["_search_score"]; !ok {
				t.Errorf("result %v missing _search_score", r["_id"])
			}
		}
		// Verify scores are in descending order.
		for i := 1; i < len(results); i++ {
			prev := results[i-1]["_search_score"].(int)
			cur := results[i]["_search_score"].(int)
			if cur > prev {
				t.Errorf("scores not descending at index %d: %d > %d", i, cur, prev)
			}
		}
	})

	t.Run("search with multiple keywords", func(t *testing.T) {
		results, err := eng.Search("docs", "programming language", nil, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		// "programming" appears in content of doc1, doc2, doc4 and title of doc1.
		// "language" appears in title+content of doc1, title+content of doc4.
		// Doc1: title "Go Programming Language" has "Programming"+"Language" = 2,
		//        content has "programming" + "language" = 2 → total 4
		// Doc4: title "Python for Data Science" none, content has "programming" + "language" = 2 → 2
		// Doc2: title "Rust Systems Programming" has "Programming" = 1,
		//        content has "programming" + no "language" = 1 → total 2
		if len(results) == 0 {
			t.Fatal("expected results, got 0")
		}
		if results[0]["_id"] != "doc1" {
			t.Fatalf("expected doc1 first (highest score), got id=%v score=%v",
				results[0]["_id"], results[0]["_search_score"])
		}
	})

	t.Run("search with limit", func(t *testing.T) {
		results, err := eng.Search("docs", "programming", nil, 2)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) > 2 {
			t.Fatalf("expected at most 2 results, got %d", len(results))
		}
	})

	t.Run("search with specific fields", func(t *testing.T) {
		// Search only in the "title" field.
		results, err := eng.Search("docs", "Go", []string{"title"}, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		// Only doc1 and doc3 have "Go" in the title.
		if len(results) != 2 {
			t.Fatalf("expected 2 results (doc1, doc3), got %d", len(results))
		}
		ids := []string{results[0]["_id"].(string), results[1]["_id"].(string)}
		sort.Strings(ids)
		if ids[0] != "doc1" || ids[1] != "doc3" {
			t.Fatalf("expected ids doc1 and doc3, got %v", ids)
		}
	})

	t.Run("search case-insensitive", func(t *testing.T) {
		results, err := eng.Search("docs", "go", nil, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("expected results for lowercase 'go', got 0")
		}
	})

	t.Run("search no match", func(t *testing.T) {
		results, err := eng.Search("docs", "zzzznotfound", nil, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})
}

func TestSearchEmptyCollection(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("empty"); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	results, err := eng.Search("empty", "anything", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected empty slice, got %d results", len(results))
	}
}

func TestSearchWithFieldFilter(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("posts"); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	doc := map[string]interface{}{
		"title":   "Go Programming",
		"content": "This is about Rust programming, not Go.",
		"tags":    []string{"go", "rust"},
	}
	if err := eng.Insert("posts", "post1", doc); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	t.Run("search only content field", func(t *testing.T) {
		results, err := eng.Search("posts", "Go", []string{"content"}, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		score := results[0]["_search_score"].(int)
		if score != 1 {
			t.Fatalf("expected score 1 (one 'Go' in content), got %d", score)
		}
	})

	t.Run("search only title field", func(t *testing.T) {
		results, err := eng.Search("posts", "Go", []string{"title"}, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		score := results[0]["_search_score"].(int)
		if score != 1 {
			t.Fatalf("expected score 1 (one 'Go' in title), got %d", score)
		}
	})

	t.Run("search with non-existent field returns no results", func(t *testing.T) {
		results, err := eng.Search("posts", "Go", []string{"nonexistent"}, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results (non-existent field), got %d", len(results))
		}
	})

	t.Run("search skips non-string fields", func(t *testing.T) {
		// The "tags" field is a []string (non-string type); it should be skipped.
		results, err := eng.Search("posts", "rust", []string{"tags"}, 10)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results (tags is non-string, should be skipped), got %d", len(results))
		}
	})
}

func TestSearchDefaultLimit(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("many"); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Insert 60 documents that all match.
	for i := 0; i < 60; i++ {
		doc := map[string]interface{}{
			"text": "keyword",
		}
		if err := eng.Insert("many", "", doc); err != nil {
			t.Fatalf("Insert doc %d: %v", i, err)
		}
	}

	// Default limit should be 50.
	results, err := eng.Search("many", "keyword", nil, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 50 {
		t.Fatalf("expected 50 results (default limit), got %d", len(results))
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	eng := newTestEngineMem(t)

	if err := eng.CreateCollection("items"); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	doc := map[string]interface{}{"name": "test"}
	if err := eng.Insert("items", "1", doc); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := eng.Search("items", "", nil, 10)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(results))
	}
}
