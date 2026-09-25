package helpers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseGitHubRemote(t *testing.T) {
	sshRemote := "git@" + "github.com:jamesread/OliveTin.git"
	sshURL := "ssh://git@" + "github.com/jamesread/OliveTin.git"
	cases := []struct {
		in          string
		owner, repo string
		ok          bool
	}{
		{sshRemote, "jamesread", "OliveTin", true},
		{"https://github.com/jamesread/OliveTin.git", "jamesread", "OliveTin", true},
		{"https://github.com/jamesread/OliveTin", "jamesread", "OliveTin", true},
		{sshURL, "jamesread", "OliveTin", true},
		{"https://gitlab.com/foo/bar.git", "", "", false},
	}
	for _, tc := range cases {
		owner, repo, ok := parseGitHubRemote(tc.in)
		if ok != tc.ok || owner != tc.owner || repo != tc.repo {
			t.Fatalf("%q => %s/%s ok=%v, want %s/%s ok=%v", tc.in, owner, repo, ok, tc.owner, tc.repo, tc.ok)
		}
	}
}

func TestGitHubHelperCacheTTL(t *testing.T) {
	dir := t.TempDir()
	h := NewGitHubHelper(dir, time.Hour, nil, nil)

	project := filepath.Join(dir, "proj")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	remote := "git@" + "github.com:acme/widgets.git"
	entry := githubCacheEntry{
		FetchedAt:  time.Now().UTC(),
		Owner:      "acme",
		Repo:       "widgets",
		FullName:   "acme/widgets",
		Topics:     []string{"go", "cli"},
		OpenIssues: 12,
		OpenPRs:    3,
		Remote:     remote,
	}
	if err := h.saveCache(project, entry); err != nil {
		t.Fatal(err)
	}

	// Simulate Collect cache hit by loading directly.
	got, hit := h.loadCache(project)
	if !hit {
		t.Fatal("expected cache hit")
	}
	meta := got.toMeta()
	if meta[FieldGitHubOwner] != "acme" || meta[FieldGitHubRepo] != "widgets" {
		t.Fatalf("meta=%#v", meta)
	}
	topics, ok := meta[FieldGitHubTopics].([]string)
	if !ok || len(topics) != 2 {
		t.Fatalf("topics=%#v", meta[FieldGitHubTopics])
	}
	if meta[FieldGitHubOpenIssues] != 12 || meta[FieldGitHubOpenPRs] != 3 {
		t.Fatalf("counts=%#v", meta)
	}
	tags, rest := SplitTags(meta)
	if len(tags) != 1 || tags[0] != GitHubTagName {
		t.Fatalf("expected GitHub tag from cache hit, got %v", tags)
	}
	if _, leaked := rest[MetaKeyTags]; leaked {
		t.Fatal("reserved tags key leaked into metadata")
	}
}

func TestGitHubBasicMetaRegistersTag(t *testing.T) {
	tags, rest := SplitTags(GitHubBasicMeta("acme", "widgets"))
	if len(tags) != 1 || tags[0] != GitHubTagName {
		t.Fatalf("tags=%v", tags)
	}
	if rest[FieldGitHubFullName] != "acme/widgets" {
		t.Fatalf("rest=%#v", rest)
	}
}

func TestGitHubRepoPageURL(t *testing.T) {
	cases := map[string]string{
		"acme/widgets": "https://github.com/acme/widgets",
		" org / repo ": "https://github.com/org/repo",
		"":             "",
		"invalid":      "",
	}
	for in, want := range cases {
		got := githubRepoPageURLFromFullName(in)
		if got != want {
			t.Fatalf("%q => %q, want %q", in, got, want)
		}
	}
	if got := GitHubPageURLFromMeta(GitHubBasicMeta("acme", "widgets")); got != "https://github.com/acme/widgets" {
		t.Fatalf("from meta: %q", got)
	}
}

func TestSplitHelperTagsNoTags(t *testing.T) {
	tags, rest := SplitTags(map[string]any{"x": 1})
	if len(tags) != 0 || rest["x"] != 1 {
		t.Fatalf("tags=%v rest=%#v", tags, rest)
	}
}

func TestGitHubHelperFieldsRegistered(t *testing.T) {
	engine := NewEngine(context.Background(), make(chan any, 1), Default(t.TempDir(), time.Hour, nil, nil))
	ids := map[string]bool{}
	for _, f := range engine.Fields() {
		ids[f.ID] = true
	}
	for _, want := range []string{
		FieldGitHubOwner, FieldGitHubRepo, FieldGitHubFullName, FieldGitHubTopics,
		FieldGitHubOpenIssues, FieldGitHubOpenPRs,
	} {
		if !ids[want] {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestParseGitHubGraphQLRepoResponse(t *testing.T) {
	const raw = `{
	  "data": {
	    "repository": {
	      "repositoryTopics": {
	        "nodes": [
	          {"topic": {"name": "go"}},
	          {"topic": {"name": "cli"}}
	        ]
	      },
	      "issues": {"totalCount": 7},
	      "pullRequests": {"totalCount": 2}
	    }
	  }
	}`
	var payload ghGraphQLRepoResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	repoData := payload.Data.Repository
	if len(repoData.RepositoryTopics.Nodes) != 2 {
		t.Fatalf("topics nodes=%d", len(repoData.RepositoryTopics.Nodes))
	}
	if repoData.Issues.TotalCount != 7 || repoData.PullRequests.TotalCount != 2 {
		t.Fatalf("counts issues=%d prs=%d", repoData.Issues.TotalCount, repoData.PullRequests.TotalCount)
	}
}
