// Package commitstore implements fit's commit history: each commit is
// saved as its own JSON file, named after its hash, and commits are
// chained via a `parent` field — the same linked-list-of-snapshots model
// real Git uses (just without merge commits / multiple parents, which
// `fit` doesn't need).
package commitstore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Commit is a single saved snapshot. Per the spec, it carries at least a
// timestamp, the author's name and email, a message, and a unique hash;
// Parent and Files make it a usable version-control commit rather than
// just a labeled backup.
type Commit struct {
	Hash        string            `json:"hash"`
	Parent      string            `json:"parent,omitempty"` // "" for the first commit
	AuthorName  string            `json:"author_name"`
	AuthorEmail string            `json:"author_email"`
	Message     string            `json:"message"`
	Timestamp   time.Time         `json:"timestamp"`
	Files       map[string]string `json:"files"` // staged path -> object hash, snapshotted at commit time
}

// ComputeHash derives the commit's unique id by hashing every field that
// makes the commit unique — including the timestamp, so two commits with
// otherwise identical content (e.g. an empty repo's very first commit
// re-run in a test) never collide.
func ComputeHash(c Commit) string {
	var b strings.Builder
	fmt.Fprintf(&b, "parent %s\n", c.Parent)
	fmt.Fprintf(&b, "author %s <%s>\n", c.AuthorName, c.AuthorEmail)
	fmt.Fprintf(&b, "timestamp %s\n", c.Timestamp.UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(&b, "message %s\n", c.Message)

	paths := make([]string, 0, len(c.Files))
	for p := range c.Files {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		fmt.Fprintf(&b, "file %s %s\n", p, c.Files[p])
	}

	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// Store wraps the commits directory (<fitDir>/commits) and the HEAD file
// (<fitDir>/refs/HEAD).
type Store struct {
	CommitsDir string
	HeadPath   string
}

func New(commitsDir, headPath string) *Store {
	return &Store{CommitsDir: commitsDir, HeadPath: headPath}
}

func (s *Store) commitPath(hash string) string {
	return filepath.Join(s.CommitsDir, hash+".json")
}

// Save persists a commit (hash must already be set) as its own JSON file.
func (s *Store) Save(c Commit) error {
	if c.Hash == "" {
		return fmt.Errorf("commit has no hash set")
	}
	if err := os.MkdirAll(s.CommitsDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.commitPath(c.Hash), data, 0o644)
}

// Load reads back a single commit by its hash.
func (s *Store) Load(hash string) (Commit, error) {
	var c Commit
	data, err := os.ReadFile(s.commitPath(hash))
	if err != nil {
		return c, fmt.Errorf("commit %s not found: %w", hash, err)
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, err
	}
	return c, nil
}

// Head returns the hash of the current HEAD commit, or "" if no commit
// has been made yet.
func (s *Store) Head() (string, error) {
	data, err := os.ReadFile(s.HeadPath)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// SetHead advances HEAD to point at the given commit hash.
func (s *Store) SetHead(hash string) error {
	if err := os.MkdirAll(filepath.Dir(s.HeadPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.HeadPath, []byte(hash+"\n"), 0o644)
}

// Log walks the commit chain starting at HEAD, following `parent`, and
// returns up to `limit` commits, newest first — exactly what `fit log`
// needs.
func (s *Store) Log(limit int) ([]Commit, error) {
	head, err := s.Head()
	if err != nil {
		return nil, err
	}
	var out []Commit
	for hash := head; hash != "" && len(out) < limit; {
		c, err := s.Load(hash)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
		hash = c.Parent
	}
	return out, nil
}
