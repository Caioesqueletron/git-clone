// Package index implements fit's staging area: which working-directory
// paths have been `fit add`ed, and which object hash (in the store) each
// one currently points at.
package index

import (
	"encoding/json"
	"os"
	"sort"
)

// Index maps a "/"-separated path, relative to the repo root, to the
// object hash `fit add` last stored for it.
type Index struct {
	Entries map[string]string `json:"entries"`
}

func New() *Index {
	return &Index{Entries: map[string]string{}}
}

// Load reads the index file at path. A missing file (e.g. right after
// `fit init`) is treated as an empty index.
func Load(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	idx := New()
	if err := json.Unmarshal(data, idx); err != nil {
		return nil, err
	}
	if idx.Entries == nil {
		idx.Entries = map[string]string{}
	}
	return idx, nil
}

// Save writes the index back to disk as JSON.
func (idx *Index) Save(path string) error {
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (idx *Index) Set(path, hash string) { idx.Entries[path] = hash }
func (idx *Index) Remove(path string)    { delete(idx.Entries, path) }

func (idx *Index) Get(path string) (string, bool) {
	h, ok := idx.Entries[path]
	return h, ok
}

// Paths returns every staged path, sorted, for deterministic output.
func (idx *Index) Paths() []string {
	out := make([]string, 0, len(idx.Entries))
	for p := range idx.Entries {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func (idx *Index) Len() int { return len(idx.Entries) }
