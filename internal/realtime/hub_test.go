package realtime

import (
	"testing"
	"time"
)

func TestHub_PublishSubscribe(t *testing.T) {
	hub := NewHub()
	hub.Start()
	defer hub.Stop()

	client := newClient("client-1", "project-a")
	hub.Register(client)
	defer hub.Unregister(client.ID)

	// Subscribe to "posts" collection with no filter.
	_, err := hub.Subscribe(client.ID, "project-a", "posts", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Publish an event.
	hub.Publish(&Event{
		Type:       EventInsert,
		Collection: "posts",
		DocID:      "doc-1",
		Data:       map[string]interface{}{"title": "hello"},
		ProjectID:  "project-a",
		Timestamp:  time.Now(),
	})

	// Wait for event to be delivered.
	select {
	case evt := <-client.events:
		if evt.DocID != "doc-1" {
			t.Fatalf("expected doc-1, got %s", evt.DocID)
		}
		if evt.Collection != "posts" {
			t.Fatalf("expected posts, got %s", evt.Collection)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Project isolation: event from different project should not reach client.
	hub.Publish(&Event{
		Type:       EventInsert,
		Collection: "posts",
		DocID:      "should-not-arrive",
		ProjectID:  "project-b",
		Timestamp:  time.Now(),
	})

	select {
	case <-client.events:
		t.Fatal("event from different project should not be delivered")
	case <-time.After(100 * time.Millisecond):
		// OK — event was correctly filtered out.
	}
}

func TestHub_Filtering(t *testing.T) {
	hub := NewHub()
	hub.Start()
	defer hub.Stop()

	client := newClient("client-filter", "project-a")
	hub.Register(client)

	// Subscribe with filter: only events where status == "active"
	_, err := hub.Subscribe(client.ID, "project-a", "tasks", map[string]interface{}{
		"status": "active",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Publish matching event.
	hub.Publish(&Event{
		Type:       EventUpdate,
		Collection: "tasks",
		DocID:      "task-1",
		Data:       map[string]interface{}{"status": "active", "title": "do it"},
		ProjectID:  "project-a",
	})

	select {
	case evt := <-client.events:
		if evt.DocID != "task-1" {
			t.Fatalf("expected task-1, got %s", evt.DocID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for matching event")
	}

	// Publish non-matching event (should be filtered out).
	hub.Publish(&Event{
		Type:       EventUpdate,
		Collection: "tasks",
		DocID:      "task-2",
		Data:       map[string]interface{}{"status": "inactive", "title": "skip"},
		ProjectID:  "project-a",
	})

	select {
	case <-client.events:
		t.Fatal("non-matching event should be filtered out")
	case <-time.After(100 * time.Millisecond):
		// OK.
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	hub := NewHub()
	hub.Start()
	defer hub.Stop()

	client := newClient("client-unsub", "project-a")
	hub.Register(client)

	sub, err := hub.Subscribe(client.ID, "project-a", "events", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Unsubscribe.
	hub.Unsubscribe(client.ID, sub.ID)

	// Publish after unsubscribe.
	hub.Publish(&Event{
		Type:       EventInsert,
		Collection: "events",
		DocID:      "after-unsub",
		ProjectID:  "project-a",
	})

	select {
	case <-client.events:
		t.Fatal("event after unsubscribe should not be delivered")
	case <-time.After(100 * time.Millisecond):
		// OK.
	}
}

func TestHub_MultiClientProjectIsolation(t *testing.T) {
	hub := NewHub()
	hub.Start()
	defer hub.Stop()

	clientA := newClient("client-a", "project-a")
	clientB := newClient("client-b", "project-b")
	hub.Register(clientA)
	hub.Register(clientB)

	_, _ = hub.Subscribe(clientA.ID, "project-a", "items", nil)
	_, _ = hub.Subscribe(clientB.ID, "project-b", "items", nil)

	// Publish to project-a.
	hub.Publish(&Event{
		Type:       EventInsert,
		Collection: "items",
		DocID:      "only-a",
		ProjectID:  "project-a",
	})

	// Client A should get it.
	select {
	case evt := <-clientA.events:
		if evt.DocID != "only-a" {
			t.Fatalf("client-a expected only-a, got %s", evt.DocID)
		}
	case <-time.After(time.Second):
		t.Fatal("client-a timeout")
	}

	// Client B should NOT get it.
	select {
	case <-clientB.events:
		t.Fatal("client-b should not receive project-a events")
	case <-time.After(100 * time.Millisecond):
		// OK.
	}
}
