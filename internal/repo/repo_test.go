package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

func TestFullWorkflow(t *testing.T) {
	dir := t.TempDir()

	r, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if filepath.Base(r.FitDir) != DefaultDirName {
		t.Fatalf("expected default dir name %q, got %q", DefaultDirName, r.FitDir)
	}

	idx, err := r.LoadIndex()
	if err != nil {
		t.Fatalf("LoadIndex: %v", err)
	}

	// Status on a brand new repo: clean, nothing tracked.
	st, err := r.Status(idx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Clean() {
		t.Errorf("expected a clean status on a fresh repo, got %+v", st)
	}

	// Add an untracked file, then check status picks it up.
	writeFile(t, dir, "hello.txt", "hello, fit\n")
	st, err = r.Status(idx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Untracked) != 1 || st.Untracked[0] != "hello.txt" {
		t.Fatalf("expected hello.txt to show as untracked, got %v", st.Untracked)
	}

	// Committing with nothing staged must fail.
	if _, err := r.Commit(idx, "empty commit", "Ada", "ada@example.com"); err == nil {
		t.Error("expected an error committing with nothing staged")
	}

	if err := r.Add(idx, []string{filepath.Join(dir, "hello.txt")}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	st, err = r.Status(idx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.StagedAdded) != 1 || st.StagedAdded[0] != "hello.txt" {
		t.Fatalf("expected hello.txt to show as staged-added, got %v", st.StagedAdded)
	}

	commit1, err := r.Commit(idx, "first commit", "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if commit1.Hash == "" {
		t.Error("expected the commit to have a non-empty hash")
	}
	if commit1.AuthorName != "Ada" || commit1.AuthorEmail != "ada@example.com" {
		t.Errorf("unexpected commit author: %+v", commit1)
	}
	if commit1.Timestamp.IsZero() {
		t.Error("expected the commit to have a non-zero timestamp")
	}

	// Right after commit, status should be clean again.
	st, err = r.Status(idx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Clean() {
		t.Errorf("expected a clean status right after commit, got %+v", st)
	}

	// Modify the file (without re-adding) -> shows up as an unstaged change.
	writeFile(t, dir, "hello.txt", "hello, fit! changed\n")
	st, err = r.Status(idx)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Modified) != 1 || st.Modified[0] != "hello.txt" {
		t.Fatalf("expected hello.txt to show as modified, got %v", st.Modified)
	}

	// Stage the change and commit again -> two commits, chained.
	if err := r.Add(idx, []string{filepath.Join(dir, "hello.txt")}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	commit2, err := r.Commit(idx, "second commit", "Ada", "ada@example.com")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if commit2.Parent != commit1.Hash {
		t.Errorf("expected commit2's parent to be commit1, got %q vs %q", commit2.Parent, commit1.Hash)
	}
	if commit2.Hash == commit1.Hash {
		t.Error("expected a different hash for a commit with different content")
	}

	log, err := r.Log(5)
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(log) != 2 {
		t.Fatalf("expected 2 commits in the log, got %d", len(log))
	}
	if log[0].Hash != commit2.Hash || log[1].Hash != commit1.Hash {
		t.Errorf("expected log order [commit2, commit1], got [%s, %s]", log[0].Hash, log[1].Hash)
	}
}

func TestInit_RefusesExistingRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(dir, ""); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	if _, err := Init(dir, ""); err == nil {
		t.Error("expected the second Init to fail: a repo already exists")
	}
}

func TestInit_CustomDirName(t *testing.T) {
	dir := t.TempDir()
	r, err := Init(dir, ".myvcs")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if filepath.Base(r.FitDir) != ".myvcs" {
		t.Errorf("expected the custom dir name to be honored, got %q", r.FitDir)
	}
	if _, err := os.Stat(filepath.Join(dir, ".myvcs")); err != nil {
		t.Errorf("expected .myvcs to exist on disk: %v", err)
	}
}

func TestFind_WalksUpParentDirectories(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root, ""); err != nil {
		t.Fatalf("Init: %v", err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	r, err := Find(nested, "")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	rootAbs, _ := filepath.Abs(root)
	if r.WorkDir != rootAbs {
		t.Errorf("expected Find to locate the repo root %q, got %q", rootAbs, r.WorkDir)
	}
}

func TestFind_NoRepoAnywhere(t *testing.T) {
	if _, err := Find(t.TempDir(), ""); err == nil {
		t.Error("expected an error when no repo exists in any parent directory")
	}
}
