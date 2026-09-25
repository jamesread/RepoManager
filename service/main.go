// Command repomanager-wui serves the RepoManager web UI and JSON API.
//
// Usage:
//
//	repomanager-wui [--basedir DIR]
//
// basedir overrides the saved scan root. With no basedir, the service
// restarts in the last used root that still exists, or the current
// directory when none does.
// Honours $PORT (default :8080).
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jamesread/RepoManager/core/cli"
	"github.com/jamesread/RepoManager/core/store"
)

func main() {
	basedir, err := cli.ParseBasedir(os.Args[1:])
	if err != nil {
		fail(err)
	}

	st, err := store.Load()
	if err != nil {
		fail(fmt.Errorf("store: %w", err))
	}

	root, err := resolveServiceRoot(st, basedir)
	if err != nil {
		fail(err)
	}

	if err := Run(root, st); err != nil {
		fail(err)
	}
}

// resolveServiceRoot picks the scan directory and records it as most recent.
// An explicit basedir wins. Otherwise the newest saved root that still
// exists is used, then the working directory.
func resolveServiceRoot(st *store.Store, basedir string) (string, error) {
	if strings.TrimSpace(basedir) == "" {
		if saved, ok := st.StartupRoot(); ok {
			return saved, st.RememberRoot(saved)
		}
	}
	root, err := cli.ResolveRoot(basedir)
	if err != nil {
		return "", err
	}
	if err := st.RememberRoot(root); err != nil {
		return "", err
	}
	return root, nil
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "repomanager-wui: %v\n", err)
	os.Exit(1)
}
