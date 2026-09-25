package helpers

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGitHubQueryState(t *testing.T) {
	g := NewGitHubQueryState()
	g.Begin()
	g.Begin()
	if g.Count() != 2 {
		t.Fatalf("count=%d", g.Count())
	}
	g.Done()
	if g.Count() != 1 {
		t.Fatalf("count=%d", g.Count())
	}
	g.Done()
	if g.Count() != 0 {
		t.Fatalf("count=%d", g.Count())
	}
}

func TestGitHubHelperCacheHitDoesNotPending(t *testing.T) {
	dir := t.TempDir()
	pending := NewGitHubQueryState()
	h := NewGitHubHelper(dir, time.Hour, pending, nil)

	project := filepath.Join(dir, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	remote := "git@" + "github.com:acme/widgets.git"
	entry := githubCacheEntry{
		FetchedAt: time.Now().UTC(),
		Owner:     "acme",
		Repo:      "widgets",
		Remote:    remote,
	}
	if err := h.saveCache(project, entry); err != nil {
		t.Fatal(err)
	}

	got, hit := h.loadCache(project)
	if !hit {
		t.Fatal("expected cache hit")
	}
	if got.Remote != remote {
		t.Fatalf("remote=%q", got.Remote)
	}
	if pending.Count() != 0 {
		t.Fatalf("cache inspection should not pending, count=%d", pending.Count())
	}
}
