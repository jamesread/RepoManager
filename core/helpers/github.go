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
	FieldGitHubIsFork      = "github.is_fork"
	FieldGitHubIsArchived  = "github.is_archived"
	FieldGitHubLatestPackage        = "github.latest_package"
	FieldGitHubLatestPackageType    = "github.latest_package_type"
	FieldGitHubLatestPackageURL     = "github.latest_package_url"

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
	IsFork      bool      `json:"is_fork"`
	IsArchived  bool      `json:"is_archived"`
	LatestPackage        string `json:"latest_package"`
	LatestPackageType    string `json:"latest_package_type"`
	LatestPackageURL     string `json:"latest_package_url"`
	Remote      string    `json:"remote"`
}

type githubRepoInfo struct {
	Topics     []string
	OpenIssues int
	OpenPRs    int
	IsFork      bool
	IsArchived  bool
	LatestPackage     githubLatestPackage
}

type githubLatestPackage struct {
	Name    string
	Type    string
	Version string
	URL     string
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
		{ID: FieldGitHubIsFork, Label: "GitHub fork", Kind: FieldBool, Sortable: true, Filterable: true},
		{ID: FieldGitHubIsArchived, Label: "GitHub archived", Kind: FieldBool, Sortable: true, Filterable: true},
		{ID: FieldGitHubLatestPackage, Label: "GitHub latest package", Kind: FieldString, Sortable: true, Filterable: true},
		{ID: FieldGitHubLatestPackageType, Label: "GitHub latest package type", Kind: FieldString, Sortable: true, Filterable: true},
		{ID: FieldGitHubLatestPackageURL, Label: "GitHub latest package URL", Kind: FieldString, Sortable: true, Filterable: true},
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
		IsFork:      info.IsFork,
		IsArchived:  info.IsArchived,
		LatestPackage:     info.LatestPackage.Name,
		LatestPackageType: info.LatestPackage.Type,
		LatestPackageURL:  info.LatestPackage.URL,
		Remote:      remote,
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
		FieldGitHubIsFork:      e.IsFork,
		FieldGitHubIsArchived:  e.IsArchived,
		FieldGitHubLatestPackage:     e.LatestPackage,
		FieldGitHubLatestPackageType: e.LatestPackageType,
		FieldGitHubLatestPackageURL:  e.LatestPackageURL,
		MetaKeyTags:            []string{GitHubTagName},
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
	Fork     bool     `json:"fork"`
	Archived bool     `json:"archived"`
	Topics   []string `json:"topics"`
}

type ghSearchResponse struct {
	TotalCount int `json:"total_count"`
}

type ghGraphQLRepoResponse struct {
	Data struct {
		Repository struct {
			IsFork     bool `json:"isFork"`
			IsArchived bool `json:"isArchived"`
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
    isFork
    isArchived
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
	info := githubRepoInfo{
		Topics:     topics,
		OpenIssues: repoData.Issues.TotalCount,
		OpenPRs:    repoData.PullRequests.TotalCount,
		IsFork:      repoData.IsFork,
		IsArchived:  repoData.IsArchived,
	}
	if pkg, err := fetchGitHubLatestPackageViaGH(ctx, owner, repo); err == nil {
		info.LatestPackage = pkg
	}
	return info, nil
}

func fetchGitHubRepoInfoHTTP(ctx context.Context, owner, repo string) (githubRepoInfo, error) {
	repoMeta, err := fetchGitHubRepoHTTP(ctx, owner, repo)
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
	info := githubRepoInfo{
		Topics:     repoMeta.Topics,
		OpenIssues: openIssues,
		OpenPRs:    openPRs,
		IsFork:      repoMeta.Fork,
		IsArchived:  repoMeta.Archived,
	}
	if pkg, err := fetchGitHubLatestPackageHTTP(ctx, owner, repo); err == nil {
		info.LatestPackage = pkg
	}
	return info, nil
}

func fetchGitHubRepoHTTP(ctx context.Context, owner, repo string) (ghRepoResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), nil)
	if err != nil {
		return ghRepoResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "RepoManager")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return ghRepoResponse{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return ghRepoResponse{}, fmt.Errorf("github api: %s", res.Status)
	}
	var payload ghRepoResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return ghRepoResponse{}, err
	}
	return payload, nil
}

type ghPackageSummary struct {
	Name        string    `json:"name"`
	PackageType string    `json:"package_type"`
	UpdatedAt   time.Time `json:"updated_at"`
	HTMLURL     string    `json:"html_url"`
	Owner       struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
}

type ghPackageVersion struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

const ghRepoPackagesGraphQLQuery = `query($owner: String!, $repo: String!) {
  repository(owner: $owner, name: $repo) {
    owner { login }
    packages(first: 100, orderBy: {field: CREATED_AT, direction: DESC}) {
      nodes {
        name
        packageType
        latestVersion { version }
      }
    }
  }
}`

type ghGraphQLPackagesResponse struct {
	Data struct {
		Repository struct {
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
			Packages struct {
				Nodes []struct {
					Name        string `json:"name"`
					PackageType string `json:"packageType"`
					LatestVersion *struct {
						Version string `json:"version"`
					} `json:"latestVersion"`
				} `json:"nodes"`
			} `json:"packages"`
		} `json:"repository"`
	} `json:"data"`
}

func fetchGitHubLatestPackageViaGH(ctx context.Context, owner, repo string) (githubLatestPackage, error) {
	packages, err := ghAPIJSON[[]ghPackageSummary](ctx, fmt.Sprintf("repos/%s/%s/packages", owner, repo))
	if err == nil {
		if pkg, resolveErr := resolveGitHubLatestPackage(ctx, packages, fetchGitHubPackageVersionsViaGH); resolveErr == nil {
			return pkg, nil
		}
	}
	if pkg, err := fetchGitHubLatestPackageViaGHGraphQL(ctx, owner, repo); err == nil {
		return pkg, nil
	}
	return fetchGitHubLatestContainerViaGHCR(ctx, owner, repo)
}

func fetchGitHubLatestPackageViaGHGraphQL(ctx context.Context, owner, repo string) (githubLatestPackage, error) {
	cmd := exec.CommandContext(ctx, "gh", "api", "graphql",
		"-f", "query="+ghRepoPackagesGraphQLQuery,
		"-f", "owner="+owner,
		"-f", "repo="+repo,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return githubLatestPackage{}, fmt.Errorf("gh api graphql packages: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var payload ghGraphQLPackagesResponse
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return githubLatestPackage{}, err
	}
	for _, node := range payload.Data.Repository.Packages.Nodes {
		version := ""
		if node.LatestVersion != nil {
			version = strings.TrimSpace(node.LatestVersion.Version)
		}
		if strings.TrimSpace(node.Name) == "" || version == "" {
			continue
		}
		pkg := ghPackageSummary{
			Name:        node.Name,
			PackageType: strings.ToLower(node.PackageType),
		}
		pkg.Owner.Login = strings.TrimSpace(payload.Data.Repository.Owner.Login)
		if pkg.Owner.Login == "" {
			pkg.Owner.Login = owner
		}
		return githubLatestPackage{
			Name:    node.Name,
			Type:    pkg.PackageType,
			Version: version,
			URL:     githubLatestPackageURL(pkg, version),
		}, nil
	}
	return githubLatestPackage{}, errNoGitHubPackages
}

func fetchGitHubLatestPackageHTTP(ctx context.Context, owner, repo string) (githubLatestPackage, error) {
	packages, err := fetchGitHubPackagesHTTP(ctx, owner, repo)
	if err == nil {
		if pkg, resolveErr := resolveGitHubLatestPackage(ctx, packages, fetchGitHubPackageVersionsHTTP); resolveErr == nil {
			return pkg, nil
		}
	}
	if _, err := exec.LookPath("gh"); err == nil {
		return fetchGitHubLatestContainerViaGHCR(ctx, owner, repo)
	}
	if err != nil {
		return githubLatestPackage{}, err
	}
	return githubLatestPackage{}, errNoGitHubPackages
}

type ghPackageVersionsFetcher func(ctx context.Context, pkg ghPackageSummary) ([]ghPackageVersion, error)

func resolveGitHubLatestPackage(ctx context.Context, packages []ghPackageSummary, fetchVersions ghPackageVersionsFetcher) (githubLatestPackage, error) {
	pkg, ok := pickLatestGitHubPackage(packages)
	if !ok {
		return githubLatestPackage{}, errNoGitHubPackages
	}
	versions, err := fetchVersions(ctx, pkg)
	if err != nil {
		return githubLatestPackage{}, err
	}
	version, ok := pickLatestGitHubPackageVersion(versions)
	if !ok {
		return githubLatestPackage{}, fmt.Errorf("github package %q has no versions", pkg.Name)
	}
	return githubLatestPackage{
		Name:    pkg.Name,
		Type:    pkg.PackageType,
		Version: version,
		URL:     githubLatestPackageURL(pkg, version),
	}, nil
}

func pickLatestGitHubPackage(packages []ghPackageSummary) (ghPackageSummary, bool) {
	var best ghPackageSummary
	var found bool
	for _, pkg := range packages {
		if strings.TrimSpace(pkg.Name) == "" {
			continue
		}
		if !found || pkg.UpdatedAt.After(best.UpdatedAt) {
			best = pkg
			found = true
		}
	}
	return best, found
}

func pickLatestGitHubPackageVersion(versions []ghPackageVersion) (string, bool) {
	var best string
	var bestAt time.Time
	var found bool
	for _, version := range versions {
		name := strings.TrimSpace(version.Name)
		if name == "" {
			continue
		}
		if !found || version.CreatedAt.After(bestAt) {
			best = name
			bestAt = version.CreatedAt
			found = true
		}
	}
	return best, found
}

func githubLatestPackageURL(pkg ghPackageSummary, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return ""
	}
	if isGitHubContainerPackage(pkg.PackageType) {
		namespace := strings.TrimSpace(pkg.Owner.Login)
		if namespace == "" {
			return ""
		}
		return githubContainerImageRef(namespace, pkg.Name, version)
	}
	return strings.TrimSpace(pkg.HTMLURL)
}

func isGitHubContainerPackage(packageType string) bool {
	switch strings.ToLower(strings.TrimSpace(packageType)) {
	case "container", "docker":
		return true
	default:
		return false
	}
}

func githubContainerImageRef(namespace, packageName, version string) string {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	packageName = strings.ToLower(strings.TrimSpace(packageName))
	version = strings.TrimSpace(version)
	if namespace == "" || packageName == "" || version == "" {
		return ""
	}
	return fmt.Sprintf("ghcr.io/%s/%s:%s", namespace, packageName, version)
}

func ghPackageVersionsAPIPath(pkg ghPackageSummary) string {
	ownerKind := "users"
	if strings.EqualFold(strings.TrimSpace(pkg.Owner.Type), "Organization") {
		ownerKind = "orgs"
	}
	login := url.PathEscape(strings.TrimSpace(pkg.Owner.Login))
	packageType := url.PathEscape(strings.TrimSpace(pkg.PackageType))
	name := url.PathEscape(strings.ToLower(strings.TrimSpace(pkg.Name)))
	return fmt.Sprintf("%s/%s/packages/%s/%s/versions", ownerKind, login, packageType, name)
}

func fetchGitHubPackageVersionsViaGH(ctx context.Context, pkg ghPackageSummary) ([]ghPackageVersion, error) {
	return ghAPIJSON[[]ghPackageVersion](ctx, ghPackageVersionsAPIPath(pkg)+"?per_page=100")
}

func ghAPIJSON[T any](ctx context.Context, endpoint string) (T, error) {
	var zero T
	cmd := exec.CommandContext(ctx, "gh", "api", endpoint)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return zero, fmt.Errorf("gh api %s: %w: %s", endpoint, err, strings.TrimSpace(stderr.String()))
	}
	var payload T
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		return zero, err
	}
	return payload, nil
}

func fetchGitHubPackagesHTTP(ctx context.Context, owner, repo string) ([]ghPackageSummary, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://api.github.com/repos/%s/%s/packages", owner, repo), nil)
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
		return nil, fmt.Errorf("github packages api: %s", res.Status)
	}
	var payload []ghPackageSummary
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
}

type ghcrTokenResponse struct {
	Token string `json:"token"`
}

type ghcrTagsResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func ghAuthCredentials(ctx context.Context) (login, token string, err error) {
	tokenCmd := exec.CommandContext(ctx, "gh", "auth", "token")
	var tokenOut, tokenErr bytes.Buffer
	tokenCmd.Stdout = &tokenOut
	tokenCmd.Stderr = &tokenErr
	if err := tokenCmd.Run(); err != nil {
		return "", "", fmt.Errorf("gh auth token: %w: %s", err, strings.TrimSpace(tokenErr.String()))
	}
	token = strings.TrimSpace(tokenOut.String())
	if token == "" {
		return "", "", errors.New("gh auth token: empty")
	}

	loginCmd := exec.CommandContext(ctx, "gh", "api", "user", "-q", ".login")
	var loginOut, loginErr bytes.Buffer
	loginCmd.Stdout = &loginOut
	loginCmd.Stderr = &loginErr
	if err := loginCmd.Run(); err != nil {
		return "", token, nil
	}
	login = strings.TrimSpace(loginOut.String())
	return login, token, nil
}

func ghcrImageRepoCandidates(owner, repo string) []string {
	o := strings.ToLower(strings.TrimSpace(owner))
	r := strings.ToLower(strings.TrimSpace(repo))
	if o == "" || r == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	add := func(imageRepo string) {
		if imageRepo == "" || seen[imageRepo] {
			return
		}
		seen[imageRepo] = true
		out = append(out, imageRepo)
	}
	add(o + "/" + r)
	if r != o {
		add(o + "/" + o)
	}
	return out
}

func fetchGitHubLatestContainerViaGHCR(ctx context.Context, owner, repo string) (githubLatestPackage, error) {
	login, token, err := ghAuthCredentials(ctx)
	if err != nil {
		return githubLatestPackage{}, err
	}
	authUser := login
	if authUser == "" {
		authUser = owner
	}
	var lastErr error
	for _, imageRepo := range ghcrImageRepoCandidates(owner, repo) {
		tags, err := fetchGHCRTags(ctx, authUser, token, imageRepo)
		if err != nil {
			lastErr = err
			continue
		}
		version := pickLatestContainerTag(tags)
		if version == "" {
			continue
		}
		namespace, pkgName, ok := strings.Cut(imageRepo, "/")
		if !ok {
			continue
		}
		pkg := ghPackageSummary{
			Name:        pkgName,
			PackageType: "container",
		}
		pkg.Owner.Login = namespace
		return githubLatestPackage{
			Name:    pkgName,
			Type:    "container",
			Version: version,
			URL:     githubLatestPackageURL(pkg, version),
		}, nil
	}
	if lastErr != nil {
		return githubLatestPackage{}, lastErr
	}
	return githubLatestPackage{}, errNoGitHubPackages
}

func fetchGHCRTags(ctx context.Context, ghUser, ghToken, imageRepo string) ([]string, error) {
	registryToken, err := fetchGHCRRegistryToken(ctx, ghUser, ghToken, imageRepo)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://ghcr.io/v2/%s/tags/list", imageRepo), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+registryToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "RepoManager")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("ghcr: repository %q not found", imageRepo)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ghcr tags api: %s", res.Status)
	}
	var payload ghcrTagsResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Tags, nil
}

func fetchGHCRRegistryToken(ctx context.Context, ghUser, ghToken, imageRepo string) (string, error) {
	scope := url.QueryEscape("repository:" + imageRepo + ":pull")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://ghcr.io/token?service=ghcr.io&scope="+scope, nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(ghUser, ghToken)
	req.Header.Set("User-Agent", "RepoManager")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ghcr token api: %s", res.Status)
	}
	var payload ghcrTokenResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return "", err
	}
	token := strings.TrimSpace(payload.Token)
	if token == "" {
		return "", errors.New("ghcr token api: empty token")
	}
	return token, nil
}

func pickLatestContainerTag(tags []string) string {
	var best string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || isFloatingContainerTag(tag) {
			continue
		}
		if best == "" || compareReleaseTag(tag, best) > 0 {
			best = tag
		}
	}
	if best != "" {
		return best
	}
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), "latest") {
			return "latest"
		}
	}
	return ""
}

func isFloatingContainerTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "latest", "stable", "main", "master", "nightly", "edge", "dev":
		return true
	default:
		return false
	}
}

func compareReleaseTag(a, b string) int {
	pa, okA := parseDotVersion(a)
	pb, okB := parseDotVersion(b)
	if okA && okB {
		return compareDotVersion(pa, pb)
	}
	return strings.Compare(a, b)
}

func parseDotVersion(tag string) ([]int, bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, false
	}
	if tag[0] == 'v' || tag[0] == 'V' {
		tag = tag[1:]
	}
	parts := strings.Split(tag, ".")
	if len(parts) == 0 {
		return nil, false
	}
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, false
		}
		n := 0
		for i := 0; i < len(part); i++ {
			if part[i] < '0' || part[i] > '9' {
				if i == 0 {
					return nil, false
				}
				break
			}
			n = n*10 + int(part[i]-'0')
		}
		out = append(out, n)
	}
	return out, true
}

func compareDotVersion(a, b []int) int {
	n := len(a)
	if len(b) > n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		av, bv := 0, 0
		if i < len(a) {
			av = a[i]
		}
		if i < len(b) {
			bv = b[i]
		}
		if av != bv {
			if av < bv {
				return -1
			}
			return 1
		}
	}
	return 0
}

func fetchGitHubPackageVersionsHTTP(ctx context.Context, pkg ghPackageSummary) ([]ghPackageVersion, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.github.com/"+ghPackageVersionsAPIPath(pkg)+"?per_page=100", nil)
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
		return nil, fmt.Errorf("github package versions api: %s", res.Status)
	}
	var payload []ghPackageVersion
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return payload, nil
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

var errNoGitHubPackages = errors.New("no github packages for repository")
