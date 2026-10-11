package helpers

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		IsFork:      true,
		IsArchived:  true,
		Remote:      remote,
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
	if meta[FieldGitHubIsFork] != true {
		t.Fatalf("is_fork=%#v", meta[FieldGitHubIsFork])
	}
	if meta[FieldGitHubIsArchived] != true {
		t.Fatalf("is_archived=%#v", meta[FieldGitHubIsArchived])
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
		FieldGitHubOpenIssues, FieldGitHubOpenPRs, FieldGitHubIsFork, FieldGitHubIsArchived,
		FieldGitHubLatestPackage, FieldGitHubLatestPackageType, FieldGitHubLatestPackageURL,
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
	      "isFork": true,
	      "isArchived": true,
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
	if !repoData.IsFork {
		t.Fatal("expected isFork=true")
	}
	if !repoData.IsArchived {
		t.Fatal("expected isArchived=true")
	}
}

func TestGitHubLatestPackageSelection(t *testing.T) {
	packages := []ghPackageSummary{
		{Name: "older", PackageType: "npm", UpdatedAt: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{
			Name:        "olivetin",
			PackageType: "container",
			UpdatedAt:   time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	packages[1].Owner.Login = "olivetin"
	packages[1].Owner.Type = "Organization"
	got, ok := pickLatestGitHubPackage(packages)
	if !ok || got.Name != "olivetin" {
		t.Fatalf("pickLatestGitHubPackage=%#v ok=%v", got, ok)
	}

	versions := []ghPackageVersion{
		{Name: "3000.19.0", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Name: "3000.20.0", CreatedAt: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)},
	}
	version, ok := pickLatestGitHubPackageVersion(versions)
	if !ok || version != "3000.20.0" {
		t.Fatalf("version=%q ok=%v", version, ok)
	}

	url := githubLatestPackageURL(got, version)
	wantURL := "ghcr.io/olivetin/olivetin:3000.20.0"
	if url != wantURL {
		t.Fatalf("url=%q want %q", url, wantURL)
	}
}

func TestPickLatestContainerTag(t *testing.T) {
	tags := []string{"0.0.7", "latest", "0.30.0", "0.29.0"}
	if got := pickLatestContainerTag(tags); got != "0.30.0" {
		t.Fatalf("tag=%q want 0.30.0", got)
	}
	if got := pickLatestContainerTag([]string{"latest"}); got != "latest" {
		t.Fatalf("tag=%q want latest", got)
	}
}

func TestGhcrImageRepoCandidates(t *testing.T) {
	got := ghcrImageRepoCandidates("JamesRead", "SickRock")
	if len(got) < 1 || got[0] != "jamesread/sickrock" {
		t.Fatalf("candidates=%v", got)
	}
	if got := ghcrImageRepoCandidates("olivetin", "OliveTin"); len(got) != 1 || got[0] != "olivetin/olivetin" {
		t.Fatalf("olive candidates=%v", got)
	}
}

func TestFetchGitHubLatestContainerViaGHCRIntegration(t *testing.T) {
	if _, err := exec.LookPath("gh"); err != nil {
		t.Skip("gh not installed")
	}
	pkg, err := fetchGitHubLatestContainerViaGHCR(context.Background(), "jamesread", "SickRock")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if pkg.Name != "sickrock" || pkg.Type != "container" {
		t.Fatalf("pkg=%+v", pkg)
	}
	if !strings.HasPrefix(pkg.URL, "ghcr.io/jamesread/sickrock:") {
		t.Fatalf("url=%q", pkg.URL)
	}
	if pkg.Version != "0.30.0" {
		t.Fatalf("version=%q want 0.30.0", pkg.Version)
	}
}

func TestGitHubLatestPackageURLNonContainer(t *testing.T) {
	pkg := ghPackageSummary{
		Name:        "widgets",
		PackageType: "npm",
		HTMLURL:     "https://github.com/acme/widgets/packages/123",
	}
	if got := githubLatestPackageURL(pkg, "1.2.3"); got != pkg.HTMLURL {
		t.Fatalf("url=%q", got)
	}
}

func TestParseGitHubRepoHTTPResponse(t *testing.T) {
	const raw = `{
	  "name": "widgets",
	  "fork": true,
	  "archived": true,
	  "topics": ["go"],
	  "owner": {"login": "acme"}
	}`
	var payload ghRepoResponse
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}
	if !payload.Fork || !payload.Archived || len(payload.Topics) != 1 || payload.Topics[0] != "go" {
		t.Fatalf("payload=%#v", payload)
	}
}
