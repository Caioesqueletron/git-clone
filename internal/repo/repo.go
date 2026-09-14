// Package repo implements fit's porcelain commands (init, add, commit,
// status, log) on top of the store/index/commitstore packages.
package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fit/internal/commitstore"
	"fit/internal/index"
	"fit/internal/store"
)

// DefaultDirName is the folder fit uses when none is given — ".fit", the
// name suggested by the challenge, but overridable (see Init/Find).
const DefaultDirName = ".fit"

// Repo bundles the working directory root and every fit subsystem,
// resolved once up front.
type Repo struct {
	WorkDir string // repo root (contains the fit dir)
	FitDir  string // e.g. <WorkDir>/.fit
	Store   *store.Store
	Commits *commitstore.Store
}

func newRepo(workDir, fitDir string) *Repo {
	return &Repo{
		WorkDir: workDir,
		FitDir:  fitDir,
		Store:   store.New(filepath.Join(fitDir, "objects")),
		Commits: commitstore.New(filepath.Join(fitDir, "commits"), filepath.Join(fitDir, "refs", "HEAD")),
	}
}

func (r *Repo) indexPath() string { return filepath.Join(r.FitDir, "INDEX") }

// Init creates a brand-new repo: the fit dir plus every subfolder each
// subsystem needs (objects/, commits/, refs/).
func Init(dir, dirName string) (*Repo, error) {
	if dirName == "" {
		dirName = DefaultDirName
	}
	fitDir := filepath.Join(dir, dirName)
	if _, err := os.Stat(fitDir); err == nil {
		return nil, fmt.Errorf("a fit repository already exists at %s", fitDir)
	}

	r := newRepo(dir, fitDir)
	for _, sub := range []string{"objects", "commits", "refs"} {
		if err := os.MkdirAll(filepath.Join(fitDir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	// An empty index up front, so `fit status` right after `fit init`
	// doesn't need to special-case "no INDEX file yet".
	if err := index.New().Save(r.indexPath()); err != nil {
		return nil, err
	}
	return r, nil
}

// Find locates the repo containing dir (or one of dir's ancestors),
// looking for a folder named dirName (walking up, the way `git` walks up
// looking for `.git`).
func Find(startDir, dirName string) (*Repo, error) {
	if dirName == "" {
		dirName = DefaultDirName
	}
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil, err
	}
	for {
		fitDir := filepath.Join(dir, dirName)
		if info, err := os.Stat(fitDir); err == nil && info.IsDir() {
			return newRepo(dir, fitDir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, fmt.Errorf("not a fit repository: no %q folder found here or in any parent directory", dirName)
		}
		dir = parent
	}
}

func (r *Repo) LoadIndex() (*index.Index, error) { return index.Load(r.indexPath()) }

// toRelPosix turns an absolute (or cwd-relative) path into a "/"-separated
// path relative to the repo root — the form used everywhere internally.
func (r *Repo) toRelPosix(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(r.WorkDir, abs)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("%s is outside the repository", path)
	}
	return filepath.ToSlash(rel), nil
}

// isFitDir reports whether a directory entry (by its absolute path) is
// this repo's fit dir, so working-directory walks can skip it.
func (r *Repo) isFitDir(absPath string) bool {
	rel, err := filepath.Rel(r.FitDir, absPath)
	return err == nil && rel == "."
}

// -------------------------------------------------------------------
// add
// -------------------------------------------------------------------

// Add copies each given path (file, or directory walked recursively) into
// the object store and stages it in the index.
func (r *Repo) Add(idx *index.Index, paths []string) error {
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			return err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return fmt.Errorf("add: %w", err)
		}
		if info.IsDir() {
			if err := r.addDir(idx, abs); err != nil {
				return err
			}
			continue
		}
		if err := r.addFile(idx, abs); err != nil {
			return err
		}
	}
	return idx.Save(r.indexPath())
}

func (r *Repo) addDir(idx *index.Index, dir string) error {
	return filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if r.isFitDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		return r.addFile(idx, path)
	})
}

func (r *Repo) addFile(idx *index.Index, abs string) error {
	content, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	hash, err := r.Store.Write(content)
	if err != nil {
		return err
	}
	rel, err := r.toRelPosix(abs)
	if err != nil {
		return err
	}
	idx.Set(rel, hash)
	return nil
}

// -------------------------------------------------------------------
// commit
// -------------------------------------------------------------------

// Commit snapshots everything currently staged into a new commit, chains
// it onto the current HEAD, and advances HEAD to it. Returns an error if
// nothing is staged, since a commit with no files wouldn't record
// anything new.
func (r *Repo) Commit(idx *index.Index, message, authorName, authorEmail string) (commitstore.Commit, error) {
	if idx.Len() == 0 {
		return commitstore.Commit{}, fmt.Errorf("nothing staged to commit — run `fit add <file>` first")
	}
	if message == "" {
		return commitstore.Commit{}, fmt.Errorf("commit message must not be empty")
	}

	parent, err := r.Commits.Head()
	if err != nil {
		return commitstore.Commit{}, err
	}

	files := make(map[string]string, idx.Len())
	for _, p := range idx.Paths() {
		h, _ := idx.Get(p)
		files[p] = h
	}

	c := commitstore.Commit{
		Parent:      parent,
		AuthorName:  authorName,
		AuthorEmail: authorEmail,
		Message:     message,
		Timestamp:   time.Now(),
		Files:       files,
	}
	c.Hash = commitstore.ComputeHash(c)

	if err := r.Commits.Save(c); err != nil {
		return commitstore.Commit{}, err
	}
	if err := r.Commits.SetHead(c.Hash); err != nil {
		return commitstore.Commit{}, err
	}
	return c, nil
}

// -------------------------------------------------------------------
// log
// -------------------------------------------------------------------

// Log returns up to `limit` commits starting at HEAD, newest first.
func (r *Repo) Log(limit int) ([]commitstore.Commit, error) {
	return r.Commits.Log(limit)
}

// -------------------------------------------------------------------
// status
// -------------------------------------------------------------------

type Status struct {
	StagedAdded    []string // staged, not in HEAD's snapshot
	StagedModified []string // staged and in HEAD, but content differs
	StagedDeleted  []string // in HEAD's snapshot, no longer staged
	Modified       []string // staged, but the working-dir copy now differs
	Deleted        []string // staged, but the working-dir file is gone
	Untracked      []string // in the working dir, never staged
}

func (r *Repo) Status(idx *index.Index) (*Status, error) {
	st := &Status{}

	head, err := r.Commits.Head()
	if err != nil {
		return nil, err
	}
	headFiles := map[string]string{}
	if head != "" {
		c, err := r.Commits.Load(head)
		if err != nil {
			return nil, err
		}
		headFiles = c.Files
	}

	// staged (index) vs HEAD -> what a commit right now would record
	for _, p := range idx.Paths() {
		h, _ := idx.Get(p)
		if hh, ok := headFiles[p]; !ok {
			st.StagedAdded = append(st.StagedAdded, p)
		} else if hh != h {
			st.StagedModified = append(st.StagedModified, p)
		}
	}
	for p := range headFiles {
		if _, ok := idx.Get(p); !ok {
			st.StagedDeleted = append(st.StagedDeleted, p)
		}
	}

	// working directory vs staged (index) -> unstaged changes + untracked
	seen := map[string]bool{}
	err = filepath.WalkDir(r.WorkDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if r.isFitDir(path) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := r.toRelPosix(path)
		if err != nil {
			return err
		}
		seen[rel] = true

		if h, ok := idx.Get(rel); ok {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if store.Hash(content) != h {
				st.Modified = append(st.Modified, rel)
			}
		} else {
			st.Untracked = append(st.Untracked, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, p := range idx.Paths() {
		if !seen[p] {
			st.Deleted = append(st.Deleted, p)
		}
	}

	sort.Strings(st.StagedAdded)
	sort.Strings(st.StagedModified)
	sort.Strings(st.StagedDeleted)
	sort.Strings(st.Modified)
	sort.Strings(st.Deleted)
	sort.Strings(st.Untracked)
	return st, nil
}

// Clean reports whether there is nothing to show in any status section.
func (st *Status) Clean() bool {
	return len(st.StagedAdded) == 0 && len(st.StagedModified) == 0 && len(st.StagedDeleted) == 0 &&
		len(st.Modified) == 0 && len(st.Deleted) == 0 && len(st.Untracked) == 0
}
