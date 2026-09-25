// Command repomanager lists Git repositories under a directory in an interactive
// terminal UI and prints the path of the repository chosen on exit.
//
// Usage:
//
//	repomanager [--basedir DIR]
//
// basedir defaults to the current directory.
// Exit status is 1 when no project was selected.
package main

import (
	"fmt"
	"os"

	"github.com/jamesread/RepoManager/core/cli"
	"github.com/jamesread/RepoManager/core/store"
)

func main() {
	basedir, err := cli.ParseBasedir(os.Args[1:])
	if err != nil {
		fail(err)
	}
	root, err := cli.ResolveRoot(basedir)
	if err != nil {
		fail(err)
	}

	st, err := store.Load()
	if err != nil {
		fail(fmt.Errorf("store: %w", err))
	}

	selected, err := Run(root, st)
	if err != nil {
		fail(err)
	}
	if selected == "" {
		os.Exit(1)
	}
	fmt.Println(selected)
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "repomanager: %v\n", err)
	os.Exit(1)
}
