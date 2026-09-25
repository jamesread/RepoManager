// Package store persists RepoManager settings, per-project tags and
// annotations, saved views and sessions, and the scan cache as JSON files
// under the XDG config and cache directories.
package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/jsonfile"
	"github.com/jamesread/RepoManager/core/project"
)

const appDirName = "plist"

// Settings lives in the user config dir: ~/.config/plist/settings.json
type Settings struct {
	TagColors           map[string]string `json:"tag_colors"`
	GitHubCacheTTLHours int               `json:"github_cache_ttl_hours"`
	// TagOrder is the global tag importance ranking (first = most important).
	// Tags not listed sort after listed ones, alphabetically.
	TagOrder []string `json:"tag_order"`
	// AnnotationKeys are the globally defined annotation keys. Any project may
	// hold a free-form string value for each key; missing means "unset".
	AnnotationKeys []string `json:"annotation_keys"`
	// Layout is the list row layout: "comfy" (default) or "compact".
	Layout string `json:"layout,omitempty"`
	// PathRelative shows project paths relative to the scan root (default true).
	PathRelative *bool `json:"path_relative,omitempty"`
	// PathTilde replaces $HOME with "~" in displayed paths (default true).
	PathTilde *bool `json:"path_tilde,omitempty"`
	// TableMode draws the list as aligned columns with a header row.
	TableMode *bool `json:"table_mode,omitempty"`
	// RootSessions stores per-scan-root list UI state (selection, filters).
	RootSessions map[string]RootSession `json:"root_sessions,omitempty"`
	// RootViews stores user-named views per scan root.
	RootViews map[string][]SavedView `json:"root_views,omitempty"`
	// RecentRoots lists scan roots the user has opened, most recent first.
	RecentRoots []string `json:"recent_roots,omitempty"`
	// TagPastels are web tag colors (#rrggbb), keyed by tag name. Assigned
	// once from the name so they stay stable across restarts.
	TagPastels map[string]string `json:"tag_pastels,omitempty"`
}

// ProjectMeta is per-project persistent data.
type ProjectMeta struct {
	Tags        []string          `json:"tags"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ProjectDB lives in ~/.config/plist/projects.json
type ProjectDB struct {
	Projects map[string]ProjectMeta `json:"projects"`
}

// CachedProject is a previously discovered project under a scan root.
type CachedProject struct {
	Path string `json:"path"`
	Rel  string `json:"rel"`
}

// ScanCache lives in ~/.cache/plist/scan.json
type ScanCache struct {
	Roots map[string][]CachedProject `json:"roots"`
}

type Store struct {
	mu       sync.Mutex
	Settings Settings
	Projects ProjectDB
	Cache    ScanCache

	configDir string
	cacheDir  string
}

func configDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

func cacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, appDirName), nil
}

// Load reads settings, the project database and the scan cache from the
// XDG config/cache directories, creating them when missing.
func Load() (*Store, error) {
	cfg, err := configDir()
	if err != nil {
		return nil, err
	}
	cache, err := cacheDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cache, 0o755); err != nil {
		return nil, err
	}

	s := &Store{
		configDir: cfg,
		cacheDir:  cache,
		Settings: Settings{
			TagColors: map[string]string{},
		},
		Projects: ProjectDB{
			Projects: map[string]ProjectMeta{},
		},
		Cache: ScanCache{
			Roots: map[string][]CachedProject{},
		},
	}

	if err := jsonfile.Read(filepath.Join(cfg, "settings.json"), &s.Settings); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.Settings.TagColors == nil {
		s.Settings.TagColors = map[string]string{}
	}
	seedDefaultTagColors(s.Settings.TagColors)
	if s.Settings.GitHubCacheTTLHours <= 0 {
		s.Settings.GitHubCacheTTLHours = 6
	}
	if s.Settings.RootSessions == nil {
		s.Settings.RootSessions = map[string]RootSession{}
	}
	if s.Settings.RootViews == nil {
		s.Settings.RootViews = map[string][]SavedView{}
	}
	if s.Settings.RecentRoots == nil {
		s.Settings.RecentRoots = []string{}
	}
	if s.Settings.TagPastels == nil {
		s.Settings.TagPastels = map[string]string{}
	}

	if err := jsonfile.Read(filepath.Join(cfg, "projects.json"), &s.Projects); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.Projects.Projects == nil {
		s.Projects.Projects = map[string]ProjectMeta{}
	}

	if err := jsonfile.Read(filepath.Join(cache, "scan.json"), &s.Cache); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if s.Cache.Roots == nil {
		s.Cache.Roots = map[string][]CachedProject{}
	}

	if err := s.PruneStaleProjects(); err != nil {
		return nil, err
	}

	_ = jsonfile.Write(filepath.Join(cfg, "settings.json"), s.Settings)

	return s, nil
}

// PruneStaleProjects removes scan-cache and project-metadata entries whose
// directories no longer exist on disk.
func (s *Store) PruneStaleProjects() error {
	s.mu.Lock()
	cacheChanged := false
	projectsChanged := false

	for root, entries := range s.Cache.Roots {
		valid := make([]CachedProject, 0, len(entries))
		for _, e := range entries {
			if project.IsIndexed(e.Path) {
				valid = append(valid, e)
				continue
			}
			cacheChanged = true
			if _, ok := s.Projects.Projects[e.Path]; ok {
				delete(s.Projects.Projects, e.Path)
				projectsChanged = true
			}
		}
		if len(valid) != len(entries) {
			s.Cache.Roots[root] = valid
		}
	}

	for path := range s.Projects.Projects {
		if project.IsIndexed(path) {
			continue
		}
		delete(s.Projects.Projects, path)
		projectsChanged = true
	}

	cache := s.Cache
	projects := s.Projects
	cachePath := s.scanCachePath()
	projectsPath := s.projectsPath()
	s.mu.Unlock()

	if cacheChanged {
		if err := jsonfile.Write(cachePath, cache); err != nil {
			return err
		}
	}
	if projectsChanged {
		if err := jsonfile.Write(projectsPath, projects); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) CacheDir() string {
	return s.cacheDir
}

func (s *Store) GitHubCacheTTL() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return helpers.GitHubTTLFromSettings(s.Settings.GitHubCacheTTLHours)
}

// TagOrder returns a copy of the global tag importance ranking.
func (s *Store) TagOrder() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.Settings.TagOrder...)
}

// SetTagOrder replaces the global tag ranking and persists it.
func (s *Store) SetTagOrder(order []string) error {
	s.mu.Lock()
	s.Settings.TagOrder = append([]string{}, order...)
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

// TagCounts returns how many projects use each tag.
func (s *Store) TagCounts() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	for _, meta := range s.Projects.Projects {
		for _, tag := range meta.Tags {
			counts[tag]++
		}
	}
	return counts
}

// AllKnownTags returns every tag referenced by any project or the ranking.
func (s *Store) AllKnownTags() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[string]struct{}{}
	var out []string
	add := func(tag string) {
		if _, ok := seen[tag]; ok || tag == "" {
			return
		}
		seen[tag] = struct{}{}
		out = append(out, tag)
	}
	for _, tag := range s.Settings.TagOrder {
		add(tag)
	}
	for _, meta := range s.Projects.Projects {
		for _, tag := range meta.Tags {
			add(tag)
		}
	}
	return out
}

// SortTagsByImportance orders tags by the global ranking, then alphabetically.
func (s *Store) SortTagsByImportance(tags []string) []string {
	return SortTagsByOrder(tags, s.TagOrder())
}

// AnnotationKeys returns the global annotation keys in definition order.
func (s *Store) AnnotationKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string{}, s.Settings.AnnotationKeys...)
}

// AddAnnotationKey defines a new global key. Returns false if it existed.
func (s *Store) AddAnnotationKey(key string) (bool, error) {
	key = NormalizeAnnotationKey(key)
	if key == "" {
		return false, nil
	}
	s.mu.Lock()
	for _, existing := range s.Settings.AnnotationKeys {
		if existing == key {
			s.mu.Unlock()
			return false, nil
		}
	}
	s.Settings.AnnotationKeys = append(s.Settings.AnnotationKeys, key)
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return true, jsonfile.Write(path, settings)
}

// AnnotationsFor returns a copy of the project's annotation values.
func (s *Store) AnnotationsFor(path string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.Projects.Projects[path]
	if !ok || len(meta.Annotations) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(meta.Annotations))
	for k, v := range meta.Annotations {
		out[k] = v
	}
	return out
}

// SetAnnotation sets (or, with an empty value, clears) one annotation on a
// project. The key is registered globally if it is new.
func (s *Store) SetAnnotation(path, key, value string) error {
	key = NormalizeAnnotationKey(key)
	value = strings.TrimSpace(value)
	if path == "" || key == "" {
		return nil
	}
	if _, err := s.AddAnnotationKey(key); err != nil {
		return err
	}

	s.mu.Lock()
	meta := s.Projects.Projects[path]
	if meta.Annotations == nil {
		meta.Annotations = map[string]string{}
	} else {
		copied := make(map[string]string, len(meta.Annotations))
		for k, v := range meta.Annotations {
			copied[k] = v
		}
		meta.Annotations = copied
	}
	if value == "" {
		delete(meta.Annotations, key)
	} else {
		meta.Annotations[key] = value
	}
	if len(meta.Annotations) == 0 {
		meta.Annotations = nil
	}
	s.Projects.Projects[path] = meta
	projects := s.Projects
	projectsPath := s.projectsPath()
	s.mu.Unlock()

	return jsonfile.Write(projectsPath, projects)
}

// Layout returns the persisted list layout name ("" when unset); the UI
// parses it into its layout mode.
func (s *Store) Layout() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Settings.Layout
}

// SetLayout persists the list layout name.
func (s *Store) SetLayout(mode string) error {
	s.mu.Lock()
	s.Settings.Layout = mode
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

func boolDefault(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// PathRelative reports whether paths are shown relative to the scan root.
func (s *Store) PathRelative() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return boolDefault(s.Settings.PathRelative, true)
}

// PathTilde reports whether $HOME is replaced with "~" in displayed paths.
func (s *Store) PathTilde() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return boolDefault(s.Settings.PathTilde, true)
}

func (s *Store) setBoolSetting(setter func(bool), v bool) error {
	s.mu.Lock()
	setter(v)
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

// SetPathRelative persists the relative-path display preference.
func (s *Store) SetPathRelative(v bool) error {
	return s.setBoolSetting(func(b bool) {
		s.Settings.PathRelative = &b
	}, v)
}

// SetPathTilde persists the tilde-path display preference.
func (s *Store) SetPathTilde(v bool) error {
	return s.setBoolSetting(func(b bool) {
		s.Settings.PathTilde = &b
	}, v)
}

// TableMode reports whether the list is drawn as aligned columns.
func (s *Store) TableMode() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return boolDefault(s.Settings.TableMode, false)
}

// SetTableMode persists the table display preference.
func (s *Store) SetTableMode(v bool) error {
	return s.setBoolSetting(func(b bool) {
		s.Settings.TableMode = &b
	}, v)
}

const maxRecentRoots = 20

// RecentRoots returns scan roots the user has opened, most recent first.
func (s *Store) RecentRoots() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Settings.RecentRoots) == 0 {
		return []string{}
	}
	return append([]string{}, s.Settings.RecentRoots...)
}

// StartupRoot returns the most recently used root that still exists on disk.
func (s *Store) StartupRoot() (string, bool) {
	for _, root := range s.RecentRoots() {
		fi, err := os.Stat(root)
		if err != nil || !fi.IsDir() {
			continue
		}
		return root, true
	}
	return "", false
}

// RememberRoot records root as the most recently used scan directory.
func (s *Store) RememberRoot(root string) error {
	normalized, err := project.NormalizeRoot(root)
	if err != nil {
		return err
	}
	s.mu.Lock()
	next := make([]string, 0, len(s.Settings.RecentRoots)+1)
	next = append(next, normalized)
	for _, existing := range s.Settings.RecentRoots {
		if existing == "" || existing == normalized {
			continue
		}
		next = append(next, existing)
		if len(next) >= maxRecentRoots {
			break
		}
	}
	s.Settings.RecentRoots = next
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

// SessionFor returns saved list UI state for a scan root.
func (s *Store) SessionFor(root string) RootSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Settings.RootSessions == nil {
		return RootSession{}
	}
	sess, ok := s.Settings.RootSessions[root]
	if !ok {
		return RootSession{}
	}
	return sess
}

// SetSession persists list UI state for a scan root.
func (s *Store) SetSession(root string, session RootSession) error {
	s.mu.Lock()
	if s.Settings.RootSessions == nil {
		s.Settings.RootSessions = map[string]RootSession{}
	}
	s.Settings.RootSessions[root] = session
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

// ViewsFor returns saved views for a scan root.
func (s *Store) ViewsFor(root string) []SavedView {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Settings.RootViews == nil {
		return nil
	}
	views := s.Settings.RootViews[root]
	out := make([]SavedView, len(views))
	copy(out, views)
	return out
}

// ViewNamed returns a saved view by name for a scan root.
func (s *Store) ViewNamed(root, name string) (SavedView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Settings.RootViews == nil {
		return SavedView{}, false
	}
	for _, v := range s.Settings.RootViews[root] {
		if v.Name == name {
			return v, true
		}
	}
	return SavedView{}, false
}

// SaveView stores or replaces a user view for a scan root. Callers are
// responsible for rejecting names that clash with predefined views.
func (s *Store) SaveView(root, name string, cfg ViewConfig) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("view name required")
	}

	s.mu.Lock()
	if s.Settings.RootViews == nil {
		s.Settings.RootViews = map[string][]SavedView{}
	}
	views := s.Settings.RootViews[root]
	updated := SavedView{Name: name, Config: cfg}
	replaced := false
	for i, v := range views {
		if v.Name == name {
			views[i] = updated
			replaced = true
			break
		}
	}
	if !replaced {
		views = append(views, updated)
	}
	s.Settings.RootViews[root] = views
	settings := s.Settings
	path := s.settingsPath()
	s.mu.Unlock()
	return jsonfile.Write(path, settings)
}

// AnnotationValueCounts returns how many projects use each non-empty value
// for the given key, across every project the store knows about.
func (s *Store) AnnotationValueCounts(key string) map[string]int {
	key = NormalizeAnnotationKey(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := map[string]int{}
	for _, meta := range s.Projects.Projects {
		if v := meta.Annotations[key]; v != "" {
			counts[v]++
		}
	}
	return counts
}

func NormalizeAnnotationKey(key string) string {
	return strings.TrimSpace(key)
}

func (s *Store) settingsPath() string { return filepath.Join(s.configDir, "settings.json") }
func (s *Store) projectsPath() string { return filepath.Join(s.configDir, "projects.json") }
func (s *Store) scanCachePath() string {
	return filepath.Join(s.cacheDir, "scan.json")
}

func (s *Store) TagsFor(path string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.Projects.Projects[path]
	if !ok {
		return nil
	}
	out := make([]string, len(meta.Tags))
	copy(out, meta.Tags)
	return out
}

// addTagLocked adds a tag in memory. Caller must hold s.mu.
func (s *Store) addTagLocked(path, tag string) bool {
	tag = NormalizeTag(tag)
	if tag == "" || path == "" {
		return false
	}
	meta := s.Projects.Projects[path]
	for _, existing := range meta.Tags {
		if existing == tag {
			return false
		}
	}
	meta.Tags = append(append([]string{}, meta.Tags...), tag)
	s.Projects.Projects[path] = meta
	return true
}

func (s *Store) ensureTagColorLocked(tag string) (color string, changed bool) {
	tag = NormalizeTag(tag)
	if tag == "" {
		return "", false
	}
	if c, ok := s.Settings.TagColors[tag]; ok && c != "" {
		return c, false
	}
	color = colorForTagName(tag)
	s.Settings.TagColors[tag] = color
	return color, true
}

func (s *Store) AddTag(path, tag string) (bool, error) {
	s.mu.Lock()
	added := s.addTagLocked(path, tag)
	_, colorChanged := s.ensureTagColorLocked(tag)
	projects := s.Projects
	settings := s.Settings
	projectsPath := s.projectsPath()
	settingsPath := s.settingsPath()
	s.mu.Unlock()

	if colorChanged {
		if err := jsonfile.Write(settingsPath, settings); err != nil {
			return added, err
		}
	}
	if !added {
		return false, nil
	}
	return true, jsonfile.Write(projectsPath, projects)
}

func (s *Store) RemoveTag(path, tag string) (bool, error) {
	tag = NormalizeTag(tag)
	if tag == "" || path == "" {
		return false, nil
	}

	s.mu.Lock()
	meta, ok := s.Projects.Projects[path]
	if !ok {
		s.mu.Unlock()
		return false, nil
	}
	idx := -1
	for i, existing := range meta.Tags {
		if existing == tag {
			idx = i
			break
		}
	}
	if idx < 0 {
		s.mu.Unlock()
		return false, nil
	}
	meta.Tags = append(append([]string{}, meta.Tags[:idx]...), meta.Tags[idx+1:]...)
	s.Projects.Projects[path] = meta
	projects := s.Projects
	projectsPath := s.projectsPath()
	s.mu.Unlock()
	return true, jsonfile.Write(projectsPath, projects)
}

// NoteGitProject records the built-in Git tag for a discovered repo (in memory).
func (s *Store) NoteGitProject(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.addTagLocked(path, "Git")
	_, _ = s.ensureTagColorLocked("Git")
}

func (s *Store) RemoveProject(path string) error {
	if path == "" {
		return nil
	}
	s.mu.Lock()
	if _, ok := s.Projects.Projects[path]; !ok {
		s.mu.Unlock()
		return nil
	}
	delete(s.Projects.Projects, path)
	projects := s.Projects
	projectsPath := s.projectsPath()
	s.mu.Unlock()
	return jsonfile.Write(projectsPath, projects)
}

func (s *Store) PersistProjectsAndSettings() error {
	s.mu.Lock()
	projects := s.Projects
	settings := s.Settings
	projectsPath := s.projectsPath()
	settingsPath := s.settingsPath()
	s.mu.Unlock()

	if err := jsonfile.Write(settingsPath, settings); err != nil {
		return err
	}
	return jsonfile.Write(projectsPath, projects)
}

func (s *Store) SetCacheRoot(root string, projects []CachedProject) error {
	s.mu.Lock()
	copied := make([]CachedProject, len(projects))
	copy(copied, projects)
	s.Cache.Roots[root] = copied
	cachePath := s.scanCachePath()
	cache := s.Cache
	s.mu.Unlock()
	return jsonfile.Write(cachePath, cache)
}

func (s *Store) CachedRoot(root string) []CachedProject {
	s.mu.Lock()
	defer s.mu.Unlock()
	cached := s.Cache.Roots[root]
	out := make([]CachedProject, len(cached))
	copy(out, cached)
	return out
}
