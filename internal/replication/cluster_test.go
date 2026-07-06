package replication

import (
	"encoding/json"
	"testing"

	"github.com/ketsuna-org/sovrabase/internal/db"
)

func TestClusterNodeMultiNamespaceReplication(t *testing.T) {
	masterEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("master engine: %v", err)
	}
	t.Cleanup(func() { masterEng.Close() })

	followerEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("follower engine: %v", err)
	}
	t.Cleanup(func() { followerEng.Close() })

	projectEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("project engine: %v", err)
	}
	t.Cleanup(func() { projectEng.Close() })

	master := NewClusterNode(&NodeConfig{Role: RoleMaster})
	master.RegisterEngine(NamespaceGlobal, masterEng)
	master.RegisterEngine("project-a", projectEng)

	follower := NewClusterNode(&NodeConfig{Role: RoleReader})
	follower.RegisterEngine(NamespaceGlobal, followerEng)
	follower.RegisterEngine("project-a", projectEng) // same mem engine simulates local copy in test

	if _, err := master.WriteNS("project-a", "posts", OpCreateCollection, "", nil); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	if _, err := master.WriteNS("project-a", "posts", OpInsert, "p1", map[string]interface{}{"title": "hello"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Simulate WAL replay on follower with a separate project engine.
	followerProjectEng, err := db.NewMemEngine()
	if err != nil {
		t.Fatalf("follower project engine: %v", err)
	}
	t.Cleanup(func() { followerProjectEng.Close() })
	follower.RegisterEngine("project-a", followerProjectEng)

	createEntry := &LogEntry{
		Namespace:  "project-a",
		Operation:  OpCreateCollection,
		Collection: "posts",
	}
	if err := follower.ApplyEntry(createEntry); err != nil {
		t.Fatalf("apply create collection: %v", err)
	}

	entry := &LogEntry{
		Namespace:  "project-a",
		Operation:  OpInsert,
		Collection: "posts",
		DocID:      "p1",
	}
	entry.Data, _ = json.Marshal(map[string]interface{}{"title": "hello"})
	if err := follower.ApplyEntry(entry); err != nil {
		t.Fatalf("apply entry: %v", err)
	}

	doc, err := followerProjectEng.Get("posts", "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if doc["title"] != "hello" {
		t.Fatalf("title = %v, want hello", doc["title"])
	}
}

func TestClusterNodeMasterEpochFencing(t *testing.T) {
	node := NewClusterNode(&NodeConfig{Role: RoleMaster})
	if err := node.BecomeMaster(); err != nil {
		t.Fatalf("become master: %v", err)
	}
	if node.MasterEpoch() != 1 {
		t.Fatalf("epoch = %d, want 1", node.MasterEpoch())
	}
}
