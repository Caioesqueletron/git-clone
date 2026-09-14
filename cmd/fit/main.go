package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"fit/internal/commitstore"
	"fit/internal/repo"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "add":
		err = cmdAdd(args)
	case "commit":
		err = cmdCommit(args)
	case "status":
		err = cmdStatus(args)
	case "log":
		err = cmdLog(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "fit: unknown command %q\n", cmd)
		usage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "fit: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `fit - a minimal, educational version control system

usage:
  fit init [dir]
  fit add <path>...
  fit commit -m <message> [-author-name <name>] [-author-email <email>]
  fit status
  fit log

Every command accepts -dir <name> to use a fit folder name other than the
default ".fit" (or set the FIT_DIR environment variable).
`)
}

// dirName resolves which fit-folder name to use: an explicit -dir flag
// (highest priority), then FIT_DIR, then the default ".fit".
func dirName(fs *flag.FlagSet) string {
	if v := fs.Lookup("dir"); v != nil && v.Value.String() != "" {
		return v.Value.String()
	}
	if v := os.Getenv("FIT_DIR"); v != "" {
		return v
	}
	return repo.DefaultDirName
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	fs.String("dir", "", "name of the fit folder to create (default \".fit\")")
	fs.Parse(args)

	target := "."
	if fs.NArg() > 0 {
		target = fs.Arg(0)
		if err := os.MkdirAll(target, 0o755); err != nil {
			return err
		}
	}

	r, err := repo.Init(target, dirName(fs))
	if err != nil {
		return err
	}
	fmt.Printf("Initialized empty fit repository in %s\n", r.FitDir)
	return nil
}

func cmdAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	fs.String("dir", "", "fit folder name")
	fs.Parse(args)

	if fs.NArg() == 0 {
		return fmt.Errorf("usage: fit add <path>...")
	}
	r, err := repo.Find(".", dirName(fs))
	if err != nil {
		return err
	}
	idx, err := r.LoadIndex()
	if err != nil {
		return err
	}
	if err := r.Add(idx, fs.Args()); err != nil {
		return err
	}
	for _, p := range fs.Args() {
		fmt.Printf("added %s\n", p)
	}
	return nil
}

func cmdCommit(args []string) error {
	fs := flag.NewFlagSet("commit", flag.ExitOnError)
	fs.String("dir", "", "fit folder name")
	message := fs.String("m", "", "commit message (required)")
	authorName := fs.String("author-name", "", "author name (defaults to FIT_AUTHOR_NAME or \"fit-user\")")
	authorEmail := fs.String("author-email", "", "author email (defaults to FIT_AUTHOR_EMAIL or \"fit-user@localhost\")")
	fs.Parse(args)

	if *message == "" {
		return fmt.Errorf("usage: fit commit -m <message>")
	}
	r, err := repo.Find(".", dirName(fs))
	if err != nil {
		return err
	}
	idx, err := r.LoadIndex()
	if err != nil {
		return err
	}

	name := resolveAuthorName(*authorName)
	email := resolveAuthorEmail(*authorEmail)

	c, err := r.Commit(idx, *message, name, email)
	if err != nil {
		return err
	}
	fmt.Printf("[%s] %s\n", shortHash(c.Hash), c.Message)
	fmt.Printf(" %d file(s) changed\n", len(c.Files))
	return nil
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	fs.String("dir", "", "fit folder name")
	fs.Parse(args)

	r, err := repo.Find(".", dirName(fs))
	if err != nil {
		return err
	}
	idx, err := r.LoadIndex()
	if err != nil {
		return err
	}
	st, err := r.Status(idx)
	if err != nil {
		return err
	}

	printSection := func(title string, paths []string) {
		if len(paths) == 0 {
			return
		}
		fmt.Println(title)
		for _, p := range paths {
			fmt.Printf("        %s\n", p)
		}
		fmt.Println()
	}

	printSection("Changes to be committed:", concat(st.StagedAdded, st.StagedModified, st.StagedDeleted))
	printSection("Changes not staged for commit:", concat(st.Modified, st.Deleted))
	printSection("Untracked files:", st.Untracked)

	if st.Clean() {
		fmt.Println("nothing to commit, working tree clean")
	}
	return nil
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("log", flag.ExitOnError)
	fs.String("dir", "", "fit folder name")
	limit := fs.Int("n", 5, "how many commits to show")
	fs.Parse(args)

	r, err := repo.Find(".", dirName(fs))
	if err != nil {
		return err
	}
	commits, err := r.Log(*limit)
	if err != nil {
		return err
	}
	if len(commits) == 0 {
		fmt.Println("no commits yet")
		return nil
	}
	for _, c := range commits {
		printCommit(c)
	}
	return nil
}

func printCommit(c commitstore.Commit) {
	fmt.Printf("commit %s\n", c.Hash)
	fmt.Printf("Author: %s <%s>\n", c.AuthorName, c.AuthorEmail)
	fmt.Printf("Date:   %s\n", c.Timestamp.Format(time.RFC1123Z))
	fmt.Println()
	for _, line := range strings.Split(strings.TrimRight(c.Message, "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println()
}

func shortHash(hash string) string {
	if len(hash) > 8 {
		return hash[:8]
	}
	return hash
}

func concat(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

func resolveAuthorName(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("FIT_AUTHOR_NAME"); v != "" {
		return v
	}
	return "fit-user"
}

func resolveAuthorEmail(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("FIT_AUTHOR_EMAIL"); v != "" {
		return v
	}
	return "fit-user@localhost"
}
