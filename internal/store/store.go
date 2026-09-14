// Package store implements fit's object store: a content-addressed
// directory of raw file copies. Every file `fit add` copies in is stored
// once, named after the SHA-256 hash of its content — so any command that
// has the hash (from the index or from a commit) can find the exact copy
// in O(1), and identical content added under different paths (or added
// twice) is only ever stored once.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Store wraps the objects directory (normally <fitDir>/objects).
type Store struct {
	Root string
}

func New(root string) *Store {
	return &Store{Root: root}
}

// Hash returns the content-addressed id a given byte slice would get,
// without writing anything.
func Hash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func (s *Store) path(hash string) string {
	return filepath.Join(s.Root, hash)
}

// Write copies content into the store under its content hash and returns
// that hash. Writing content that's already stored is a cheap no-op.
func (s *Store) Write(content []byte) (string, error) {
	hash := Hash(content)
	dest := s.path(hash)
	if _, err := os.Stat(dest); err == nil {
		return hash, nil // already stored
	}

	if err := os.MkdirAll(s.Root, 0o755); err != nil {
		return "", err
	}
	// Write to a temp file then rename, so a crash mid-write never leaves
	// a partially-written object behind under its final name.
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return hash, nil
}

// Read returns the stored content for a given hash.
func (s *Store) Read(hash string) ([]byte, error) {
	content, err := os.ReadFile(s.path(hash))
	if err != nil {
		return nil, fmt.Errorf("object %s not found: %w", hash, err)
	}
	return content, nil
}

// Exists reports whether a given hash is already stored.
func (s *Store) Exists(hash string) bool {
	_, err := os.Stat(s.path(hash))
	return err == nil
}
