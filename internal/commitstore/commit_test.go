package commitstore

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return New(filepath.Join(dir, "commits"), filepath.Join(dir, "refs", "HEAD"))
}

func TestSaveLoadRoundtrip(t *testing.T) {
	s := newTestStore(t)

	c := Commit{
		AuthorName:  "Ada Lovelace",
		AuthorEmail: "ada@example.com",
		Message:     "first commit",
		Timestamp:   time.Now(),
		Files:       map[string]string{"a.txt": "hash-a"},
	}
	c.Hash = ComputeHash(c)

	if err := s.Save(c); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.Load(c.Hash)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Message != c.Message || got.AuthorName != c.AuthorName {
		t.Errorf("loaded commit doesn't match: %+v", got)
	}
}

func TestComputeHash_UniquePerContent(t *testing.T) {
	base := Commit{
		AuthorName:  "A",
		AuthorEmail: "a@example.com",
		Timestamp:   time.Unix(1700000000, 0),
		Files:       map[string]string{"a.txt": "h1"},
	}
	c1 := base
	c1.Message = "message one"
	c2 := base
	c2.Message = "message two"

	if ComputeHash(c1) == ComputeHash(c2) {
		t.Error("expected different messages to produce different hashes")
	}
	if ComputeHash(c1) != ComputeHash(c1) {
		t.Error("expected the same commit content to produce the same hash")
	}
}

func TestHeadDefaultsToEmpty(t *testing.T) {
	s := newTestStore(t)
	head, err := s.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head != "" {
		t.Errorf("expected an empty HEAD on a fresh store, got %q", head)
	}
}

func TestSetHeadThenHead(t *testing.T) {
	s := newTestStore(t)
	if err := s.SetHead("abc123"); err != nil {
		t.Fatalf("SetHead: %v", err)
	}
	head, err := s.Head()
	if err != nil {
		t.Fatalf("Head: %v", err)
	}
	if head != "abc123" {
		t.Errorf("expected HEAD to be abc123, got %q", head)
	}
}

func TestLog_WalksParentChainNewestFirst(t *testing.T) {
	s := newTestStore(t)

	var parent string
	var hashes []string
	for i := 0; i < 7; i++ {
		c := Commit{
			Parent:      parent,
			AuthorName:  "A",
			AuthorEmail: "a@example.com",
			Message:     "commit",
			Timestamp:   time.Now().Add(time.Duration(i) * time.Second),
			Files:       map[string]string{},
		}
		c.Hash = ComputeHash(c)
		if err := s.Save(c); err != nil {
			t.Fatalf("Save: %v", err)
		}
		if err := s.SetHead(c.Hash); err != nil {
			t.Fatalf("SetHead: %v", err)
		}
		hashes = append(hashes, c.Hash)
		parent = c.Hash
	}

	log, err := s.Log(5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(log) != 5 {
		t.Fatalf("expected 5 commits (out of 7 made), got %d", len(log))
	}
	// hashes[6] is the newest (last made); log[0] should be the newest.
	for i := 0; i < 5; i++ {
		want := hashes[6-i]
		if log[i].Hash != want {
			t.Errorf("log[%d] = %s, want %s (newest-first order)", i, log[i].Hash, want)
		}
	}
}

func TestLog_EmptyRepo(t *testing.T) {
	s := newTestStore(t)
	log, err := s.Log(5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(log) != 0 {
		t.Errorf("expected no commits on an empty repo, got %d", len(log))
	}
}
