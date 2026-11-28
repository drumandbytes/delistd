package web

import (
	"testing"
	"time"

	"github.com/drumandbytes/delistd/internal/config"
)

func TestSessionStoreLifecycle(t *testing.T) {
	store := NewSessionStore(time.Hour)

	id, err := store.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(id) != 64 { // 32 random bytes, hex-encoded
		t.Fatalf("session id looks wrong: %q (len %d)", id, len(id))
	}

	if got := store.Get(id); got == nil || got.ID != id {
		t.Fatalf("Get after Create returned %+v", got)
	}
	if store.Get("nope") != nil {
		t.Error("Get with unknown id should be nil")
	}
	if store.Get("") != nil {
		t.Error("Get with empty id should be nil")
	}

	ok := store.Update(id, func(s *Session) {
		s.Step = "email"
		s.Profile = config.Profile{FirstName: "Ada"}
		s.ManualSend = true
	})
	if !ok {
		t.Fatal("Update returned false for a live session")
	}
	got := store.Get(id)
	if got.Step != "email" || got.Profile.FirstName != "Ada" || !got.ManualSend {
		t.Fatalf("Update did not persist: %+v", got)
	}
	if store.Update("nope", func(*Session) {}) {
		t.Error("Update on unknown id should return false")
	}

	store.Delete(id)
	if store.Get(id) != nil {
		t.Error("Get after Delete should be nil")
	}
}

func TestSessionExpiry(t *testing.T) {
	store := NewSessionStore(20 * time.Millisecond)

	id, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}
	if store.Get(id) == nil {
		t.Fatal("session should be live immediately after Create")
	}

	time.Sleep(40 * time.Millisecond)

	if store.Get(id) != nil {
		t.Error("expired session should not be returned by Get")
	}
	if store.Update(id, func(*Session) {}) {
		t.Error("expired session should not be updatable")
	}
}

func TestSessionUpdateExtendsExpiry(t *testing.T) {
	// Sleeps well under the TTL so a slow runner (Windows timers overshoot) cannot expire it
	// between updates; four of them still add up to more than the TTL.
	store := NewSessionStore(400 * time.Millisecond)

	id, err := store.Create()
	if err != nil {
		t.Fatal(err)
	}

	// Keep touching it past the original TTL; each Update should push the
	// expiry out, so it stays alive.
	for i := 0; i < 4; i++ {
		time.Sleep(120 * time.Millisecond)
		if !store.Update(id, func(s *Session) { s.Step = "still here" }) {
			t.Fatalf("session expired despite being updated (iteration %d)", i)
		}
	}
}

func TestExpiredSessionsAreSweptOnCreate(t *testing.T) {
	store := NewSessionStore(10 * time.Millisecond)
	for i := 0; i < 5; i++ {
		if _, err := store.Create(); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(30 * time.Millisecond)
	if _, err := store.Create(); err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	remaining := len(store.byID)
	store.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("%d sessions left, want only the new one", remaining)
	}
}

func TestSessionIDsAreUnique(t *testing.T) {
	store := NewSessionStore(time.Hour)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id, err := store.Create()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != 64 || seen[id] {
			t.Fatalf("bad or duplicate session id %q", id)
		}
		seen[id] = true
	}
}

// Get hands out a copy, so a reader is never looking at a session that
// Update is changing in another request.
func TestGetReturnsASnapshot(t *testing.T) {
	store := NewSessionStore(time.Hour)
	id, _ := store.Create()
	snap := store.Get(id)
	store.Update(id, func(s *Session) { s.Step = "changed" })
	if snap.Step != "" {
		t.Errorf("snapshot changed under the reader: %q", snap.Step)
	}
	if store.Get(id).Step != "changed" {
		t.Error("the update was lost")
	}
}
