package store

import "testing"

func TestWriteReadRoundtrip(t *testing.T) {
	s := New(t.TempDir())

	hash, err := s.Write([]byte("hello, fit"))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(hash) != 64 { // sha256 hex digest length
		t.Fatalf("expected a 64-char hex sha256, got %d chars: %q", len(hash), hash)
	}

	got, err := s.Read(hash)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != "hello, fit" {
		t.Errorf("content mismatch: got %q", got)
	}
	if !s.Exists(hash) {
		t.Error("expected Exists to be true right after Write")
	}
}

func TestSameContentSameHash(t *testing.T) {
	s := New(t.TempDir())
	h1, _ := s.Write([]byte("identical"))
	h2, _ := s.Write([]byte("identical"))
	if h1 != h2 {
		t.Errorf("expected identical content to produce identical hashes, got %s and %s", h1, h2)
	}
}

func TestReadMissingObject(t *testing.T) {
	s := New(t.TempDir())
	if _, err := s.Read("does-not-exist"); err == nil {
		t.Error("expected an error reading a missing object")
	}
	if s.Exists("does-not-exist") {
		t.Error("expected Exists to be false for a missing object")
	}
}
