package streamsh

import (
	"testing"
)

func TestStoreCreateAndList(t *testing.T) {
	s := NewStore()
	sess := s.Create("test-session", 100)

	if sess.Title != "test-session" {
		t.Errorf("title = %q, want %q", sess.Title, "test-session")
	}
	if !sess.Connected {
		t.Error("expected connected=true")
	}
	if len(sess.ShortID) != 8 {
		t.Errorf("short ID length = %d, want 8", len(sess.ShortID))
	}

	list := s.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 session, got %d", len(list))
	}
}

func TestStoreGet(t *testing.T) {
	s := NewStore()
	sess := s.Create("get-test", 100)

	found, ok := s.Get(sess.ID)
	if !ok || found.ID != sess.ID {
		t.Error("expected to find session by ID")
	}
}

func TestStoreFindByPrefix(t *testing.T) {
	s := NewStore()
	sess := s.Create("prefix-test", 100)

	found, err := s.FindByPrefix(sess.ShortID[:4])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.ID != sess.ID {
		t.Error("wrong session returned")
	}
}

func TestStoreFindByPrefixAmbiguous(t *testing.T) {
	s := NewStore()
	s.Create("a", 100)
	s.Create("b", 100)

	// Using empty prefix matches all -> ambiguous
	_, err := s.FindByPrefix("")
	if err == nil {
		t.Error("expected ambiguous error")
	}
}

func TestStoreFindByTitle(t *testing.T) {
	s := NewStore()
	s.Create("My Session", 100)

	found, err := s.FindByTitle("my session") // case insensitive
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.Title != "My Session" {
		t.Errorf("title = %q, want %q", found.Title, "My Session")
	}
}

func TestStoreResolve(t *testing.T) {
	s := NewStore()
	sess := s.Create("dev-server", 100)

	// By full UUID
	found, err := s.Resolve(sess.ID.String())
	if err != nil || found.ID != sess.ID {
		t.Error("resolve by UUID failed")
	}

	// By prefix
	found, err = s.Resolve(sess.ShortID[:4])
	if err != nil || found.ID != sess.ID {
		t.Error("resolve by prefix failed")
	}

	// By title
	found, err = s.Resolve("dev-server")
	if err != nil || found.ID != sess.ID {
		t.Error("resolve by title failed")
	}

	// Not found
	_, err = s.Resolve("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent")
	}
}

func TestStoreRemove(t *testing.T) {
	s := NewStore()
	sess := s.Create("to-remove", 100)
	s.Remove(sess.ID)

	if len(s.List()) != 0 {
		t.Error("expected empty store after remove")
	}
}
