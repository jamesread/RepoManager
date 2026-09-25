package helpers

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jamesread/RepoManager/core/jsonfile"
)

const (
	FieldGitHubOwner      = "github.owner"
	FieldGitHubRepo       = "github.repo"
	FieldGitHubFullName   = "github.full_name"
	FieldGitHubTopics     = "github.topics"
	FieldGitHubOpenIssues = "github.open_issues"
	FieldGitHubOpenPRs    = "github.open_prs"

	defaultGitHubCacheTTL = 6 * time.Hour
)

type GitHubHelper struct {
	cacheDir        string
	ttl             time.Duration
	pending         *GitHubQueryState
	onPendingChange func()
}

type githubCacheEntry struct {
	FetchedAt  time.Time `json:"fetched_at"`
	Owner      string    `json:"owner"`
	Repo       string    `json:"repo"`
	FullName   string    `json:"full_name"`
	Topics     []string  `json:"topics"`
	OpenIssues int       `json:"open_issues"`
	OpenPRs    int       `json:"open_prs"`
	Remote     string    `json:"remote"`
}

type githubRepoInfo struct {
	Topics     []string
	OpenIssues int
	OpenPRs    int
}

func NewGitHubHelper(cacheDir string, ttl time.Duration, pending *GitHubQueryState, onPendingChange func()) *GitHubHelper {
	if ttl <= 0 {
		ttl = defaultGitHubCacheTTL
	}
	return &GitHubHelper{
		cacheDir:        filepath.Join(cacheDir, "github"),
		ttl:             ttl,
		pending:         pending,
		onPendingChange: onPendingChange,
	}
}

func (h *GitHubHelper) Name() string { return "github" }

func (h *GitHubHelper) Fields() []Field {
	return []Field{
		{ID: FieldGitHubOwner, Label: "GitHub owner", Kind: FieldString, Sortable: true, Filterable: true},
		{ID: FieldGitHubRepo, Label: "GitHub repo", Kind: FieldString, Sortable: true, Filterable: true},
		{ID: FieldGitHubFullName, Label: "GitHub full name", Kind: FieldString, Sortable: true, Filterable: true},
		{ID: FieldGitHubTopics, Label: "GitHub topics", Kind: FieldMulti, Sortable: false, Filterable: true},
		{ID: FieldGitHubOpenIssues, Label: "GitHub open issues", Kind: FieldInt, Sortable: true, Filterable: true},
		{ID: FieldGitHubOpenPRs, Label: "GitHub open PRs", Kind: FieldInt, Sortable: true, Filterable: true},
	}
}

func (h *GitHubHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	remote, err := gitOriginURL(ctx, projectPath)
	if err != nil || remote == "" {
		return map[string]any{}, nil
	}
	owner, repo, ok := parseGitHubRemote(remote)
	if !ok {
		return map[string]any{}, nil
	}

	if entry, hit := h.loadCache(projectPath); hit {
		if entry.Remote == remote && time.Since(entry.FetchedAt) < h.ttl {
			return entry.toMeta(), nil
		}
	}

	var entry githubCacheEntry
	err = h.withPending(func() error {
		var fetchErr error
		entry, fetchErr = h.fetch(ctx, projectPath, remote, owner, repo)
		return fetchErr
	})
	if err != nil {
		// Fall back to remote-parsed owner/repo without topics.
		return GitHubBasicMeta(owner, repo), nil
	}
	return entry.toMeta(), nil
}

func (h *GitHubHelper) withPending(fn func() error) error {
	if h.pending == nil {
		return fn()
	}
	h.pending.Begin()
	if h.onPendingChange != nil {
		h.onPendingChange()
	}
	defer func() {
		h.pending.Done()
		if h.onPendingChange != nil {
			h.onPendingChange()
		}
	}()
	return fn()
}

// GitHubTagName is registered on any project whose origin points at GitHub.
const GitHubTagName = "GitHub"

// GitHubBasicMeta is what we know from the remote URL alone.
func GitHubBasicMeta(owner, repo string) map[string]any {
	return map[string]any{
		FieldGitHubOwner:    owner,
		FieldGitHubRepo:     repo,
		FieldGitHubFullName: owner + "/" + repo,
		MetaKeyTags:         []string{GitHubTagName},
	}
}

func githubRepoPageURL(owner, repo string) string {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	if owner == "" || repo == "" {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s", owner, repo)
}

func githubRepoPageURLFromFullName(full string) string {
	full = strings.TrimSpace(full)
	if full == "" {
		return ""
	}
	owner, repo, ok := strings.Cut(full, "/")
	if !ok {
		return ""
	}
	return githubRepoPageURL(owner, repo)
}

func GitHubPageURLFromMeta(meta map[string]any) string {
	if meta == nil {
		return ""
	}
	if owner, ok := meta[FieldGitHubOwner].(string); ok {
		if repo, ok := meta[FieldGitHubRepo].(string); ok {
			if u := githubRepoPageURL(owner, repo); u != "" {
				return u
			}
		}
	}
	if full, ok := meta[FieldGitHubFullName].(string); ok {
		return githubRepoPageURLFromFullName(full)
	}
	return ""
}

func (h *GitHubHelper) CollectForced(ctx context.Context, projectPath string) (map[string]any, error) {
	remote, err := gitOriginURL(ctx, projectPath)
	if err != nil || remote == "" {
		return map[string]any{}, nil
	}
	owner, repo, ok := parseGitHubRemote(remote)
	if !ok {
		return map[string]any{}, nil
	}
	var entry githubCacheEntry
	err = h.withPending(func() error {
		var fetchErr error
		entry, fetchErr = h.fetch(ctx, projectPath, remote, owner, repo)
		return fetchErr
	})
	if err != nil {
		return GitHubBasicMeta(owner, repo), err
	}
	return entry.toMeta(), nil
}

func (h *GitHubHelper) fetch(ctx context.Context, projectPath, remote, owner, repo string) (githubCacheEntry, error) {
	info, err := fetchGitHubRepoInfo(ctx, owner, repo)
	if err != nil {
		return githubCacheEntry{}, err
	}
	entry := githubCacheEntry{
		FetchedAt:  time.Now().UTC(),
		Owner:      owner,
		Repo:       repo,
		FullName:   owner + "/" + repo,
		Topics:     info.Topics,
		OpenIssues: info.OpenIssues,
		OpenPRs:    info.OpenPRs,
		Remote:     remote,
	}
	_ = h.saveCache(projectPath, entry)
	return entry, nil
}

func (e githubCacheEntry) toMeta() map[string]any {
	topics := append([]string{}, e.Topics...)
	return map[string]any{
		FieldGitHubOwner:      e.Owner,
		FieldGitHubRepo:       e.Repo,
		FieldGitHubFullName:   e.FullName,
		FieldGitHubTopics:     topics,
		FieldGitHubOpenIssues: e.OpenIssues,
		FieldGitHubOpenPRs:    e.OpenPRs,
		MetaKeyTags:           []string{GitHubTagName},
	}
}

func (h *GitHubHelper) cachePath(projectPath string) string {
	sum := sha1.Sum([]byte(projectPath))
	return filepath.Join(h.cacheDir, hex.EncodeToString(sum[:])+".json")
}

func (h *GitHubHelper) loadCache(projectPath string) (githubCacheEntry, bool) {
	var entry githubCacheEntry
	if err := jsonfile.Read(h.cachePath(projectPath), &entry); err != nil {
		return githubCacheEntry{}, false
	}
	return entry, true
}

func (h *GitHubHelper) saveCache(projectPath string, entry githubCacheEntry) error {
	if err := os.MkdirAll(h.cacheDir, 0o755); err != nil {
		return err
	}
	return jsonfile.Write(h.cachePath(projectPath), entry)
}

func gitOriginURL(ctx context.Context, projectPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "remote", "get-url", "origin")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func parseGitHubRemote(remote string) (owner, repo string, ok bool) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")

	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		remote = strings.TrimPrefix(remote, "git@github.com:")
	case strings.HasPrefix(strings.ToLower(remote), "ssh://git@github.com/"):
		// Preserve original casing after the prefix strip using index.
		idx := strings.Index(strings.ToLower(remote), "ssh://git@github.com/")
		remote = remote[idx+len("ssh://git@github.com/"):]
	case strings.Contains(strings.ToLower(remote), "github.com/"):
		idx := strings.Index(strings.ToLower(remote), "github.com/")
		remote = remote[idx+len("github.com/"):]
	default:
		return "", "", false
	}

	remote = strings.Trim(remote, "/")
	parts := strings.Split(remote, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

type ghRepoResponse struct {
	Owner struct {
		Login string `json:"login"`
	} `json:"owner"`
	Name   string   `json:"name"`
	Topics []string `json:"topics"`
}

type ghSearchResponse struct {
	TotalCount int `json:"total_count"`
}

type ghGraphQLRepoResponse struct {
	Data struct {
		Repository struct {
			RepositoryTopics struct {
				Nodes []struct {
					Topic struct {
						Name string `json:"name"`
					} `json:"topic"`
				} `json:"nodes"`
			} `json:"repositoryTopics"`
			Issues struct {
				TotalCount int `json:"totalCount"`
			} `json:"issues"`
			PullRequests struct {
				TotalCount int `json:"totalCount"`
			} `json:"pullRequests"`
		} `json:"repository"`
	} `json:"data"`
}

func fetchGitHubRepoInfo(ctx context.Context, owner, repo string) (githubRepoInfo, error) {
	if _, err := exec.LookPath("gh"); err == nil {
		info, err := fetchGitHubRepoInfoViaGH(ctx, owner, repo)
		if err == nil {
			return info, nil
		}
		// Fall through to HTTP if gh fails (e.g. not authenticated for this host).
	}
	return fetchGitHubRepoInfoHTTP(ctx, owner, repo)
}

const ghRepoGraphQLQuery = `query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    repositoryTopics(first: 100) { nodes { topic { name } } }
    issues(states: OPEN) { totalCount }
    pullRequests(states: OPEN) { totalCount }
  }
}`

func fetchGitHubRepoInfoViaGH(ctx context.Context, owner, repo string) (githubRepoInfo, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "graphql",
		"-f", "query="+ghRepoGraphQLQuery,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return githubRepoInfo{}, fmt.Errorf("gh api graphql: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var payload ghGraphQLRepoResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return githubRepoInfo{}, err
	}
	repoData := payload.Data.Repository
	topics := make([]string, 0, len(repoData.RepositoryTopics.Nodes))
	for _, node := range repoData.RepositoryTopics.Nodes {
		if name := strings.TrimSpace(node.Topic.Name); name != "" {
			topics = append(topics, name)
		}
	}
	return githubRepoInfo{
		Topics:     topics,
		OpenIssues: repoData.Issues.TotalCount,
		OpenPRs:    repoData.PullRequests.TotalCount,
	}, nil
}

func fetchGitHubRepoInfoHTTP(ctx context.Context, owner, repo string) (githubRepoInfo, error) {
	topics, err := fetchGitHubTopicsHTTP(ctx, owner, repo)
	if err != nil {
		return githubRepoInfo{}, err
	}
	openIssues, err := fetchGitHubSearchCountHTTP(ctx, fmt.Sprintf("repo:%s/%s is:issue is:open", owner, repo))
	if err != nil {
		return githubRepoInfo{}, err
	}
	openPRs, err := fetchGitHubSearchCountHTTP(ctx, fmt.Sprintf("repo:%s/%s is:pr is:open", owner, repo))
	if err != nil {
		return githubRepoInfo{}, err
	}
	return githubRepoInfo{
		Topics:     topics,
		OpenIssues: openIssues,
		OpenPRs:    openPRs,
	}, nil
}

func fetchGitHubTopicsHTTP(ctx context.Context, owner, repo string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RepoManager")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api: %s", res.Status)
	}
	var payload ghRepoResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Topics, nil
}

func fetchGitHubSearchCountHTTP(ctx context.Context, query string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/search/issues?q="+url.QueryEscape(query), nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RepoManager")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("github search api: %s", res.Status)
	}
	var payload ghSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return 0, err
	}
	return payload.TotalCount, nil
}

func GitHubTTLFromSettings(hours int) time.Duration {
	if hours <= 0 {
		return defaultGitHubCacheTTL
	}
	return time.Duration(hours) * time.Hour
}

// Ensure errors package retained for callers that wrap missing remotes.
var errNoGitHubRemote = errors.New("no github origin remote")
