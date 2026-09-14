package index

import (
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsEmptyIndex(t *testing.T) {
	idx, err := Load(filepath.Join(t.TempDir(), "INDEX"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if idx.Len() != 0 {
		t.Errorf("expected an empty index, got %d entries", idx.Len())
	}
}

func TestSetSaveLoadRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "INDEX")

	idx := New()
	idx.Set("a.txt", "hash-a")
	idx.Set("dir/b.txt", "hash-b")
	if err := idx.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", loaded.Len())
	}
	if h, ok := loaded.Get("a.txt"); !ok || h != "hash-a" {
		t.Errorf("a.txt: got hash=%q ok=%v", h, ok)
	}
	if h, ok := loaded.Get("dir/b.txt"); !ok || h != "hash-b" {
		t.Errorf("dir/b.txt: got hash=%q ok=%v", h, ok)
	}
}

func TestRemove(t *testing.T) {
	idx := New()
	idx.Set("a.txt", "hash-a")
	idx.Remove("a.txt")
	if _, ok := idx.Get("a.txt"); ok {
		t.Error("expected a.txt to be gone after Remove")
	}
}

func TestPathsSorted(t *testing.T) {
	idx := New()
	idx.Set("z.txt", "h")
	idx.Set("a.txt", "h")
	idx.Set("m.txt", "h")
	paths := idx.Paths()
	want := []string{"a.txt", "m.txt", "z.txt"}
	if len(paths) != len(want) {
		t.Fatalf("expected %d paths, got %d", len(want), len(paths))
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("paths[%d] = %q, want %q", i, paths[i], want[i])
		}
	}
}
