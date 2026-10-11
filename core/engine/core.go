package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"

	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/project"
	"github.com/jamesread/RepoManager/core/store"
)

var (
	// ErrProjectNotFound means the path is not a project in the current scan.
	ErrProjectNotFound = errors.New("project not found")
	// ErrLocalChanges means the project has the git "changes" tag.
	ErrLocalChanges = errors.New("project has local changes")
	// ErrReservedTag means the tag is assigned by a helper, not by the user.
	ErrReservedTag = errors.New("tag is set by a helper")
)

// Core is the in-process Engine implementation.
type Core struct {
	mu     sync.Mutex
	store  *store.Store
	root   string
	all    []Project
	found  int
	scanOn bool
	// scanGen increments when the root changes so in-flight scans from a
	// previous root stop applying results.
	scanGen uint64

	ctx    context.Context
	cancel context.CancelFunc

	helpers       *helpers.Engine
	helperEvents  chan any
	githubPending *helpers.GitHubQueryState

	subsMu sync.Mutex
	subs   []chan Event

	wg     sync.WaitGroup // drainHelpers + prune emits
	scanWG sync.WaitGroup // live filesystem scan
}

// New builds a Core for root and loads the scan cache into memory.
// Call Subscribe, then Start (or Send(ScanRequest{})) so the UI does not
// miss early events.
func New(st *store.Store, root string) *Core {
	ctx, cancel := context.WithCancel(context.Background())
	helperEvents := make(chan any, 128)
	ghPending := helpers.NewGitHubQueryState()
	c := &Core{
		store:         st,
		root:          root,
		ctx:           ctx,
		cancel:        cancel,
		helperEvents:  helperEvents,
		githubPending: ghPending,
	}
	c.helpers = helpers.NewEngine(ctx, helperEvents, helpers.Default(st.CacheDir(), st.GitHubCacheTTL(), ghPending, func() {
		c.emit(GitHubPending{Count: ghPending.Count()})
	}))
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.drainHelpers()
	}()
	c.loadCachedProjects()
	return c
}

// Start begins a live filesystem scan. Safe to call once after Subscribe.
func (c *Core) Start() {
	c.startScan()
}

// Rescan walks the current root again. Projects whose directories are gone
// are removed when the scan finishes.
func (c *Core) Rescan(ctx context.Context) error {
	done := c.startScan()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-c.ctx.Done():
		return c.ctx.Err()
	}
}

func (c *Core) Subscribe() <-chan Event {
	ch := make(chan Event, 128)
	c.subsMu.Lock()
	c.subs = append(c.subs, ch)
	c.subsMu.Unlock()
	return ch
}

func (c *Core) emit(ev Event) {
	c.subsMu.Lock()
	subs := append([]chan Event{}, c.subs...)
	c.subsMu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- ev:
		default:
			// Drop rather than block producers when a UI is not draining.
		}
	}
}

func (c *Core) Snapshot() []Project {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]Project, len(c.all))
	for i, p := range c.all {
		out[i] = p.clone()
	}
	return out
}

func (c *Core) Fields() []helpers.Field {
	if c.helpers == nil {
		return nil
	}
	return c.helpers.Fields()
}

func (c *Core) HelperCount() int {
	if c.helpers == nil {
		return 0
	}
	return c.helpers.HelperCount()
}

func (c *Core) GitHubPendingCount() int {
	if c.githubPending == nil {
		return 0
	}
	return c.githubPending.Count()
}

func (c *Core) Store() *store.Store { return c.store }
func (c *Core) Root() string        { return c.root }

func (c *Core) Scanning() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.scanOn
}

func (c *Core) Found() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.found
}

func (c *Core) Close() {
	c.cancel()
	c.scanWG.Wait()
	if c.helpers != nil {
		c.helpers.Wait()
	}
	c.wg.Wait()
	c.subsMu.Lock()
	for _, ch := range c.subs {
		close(ch)
	}
	c.subs = nil
	c.subsMu.Unlock()
}

func (c *Core) Send(ctx context.Context, r Request) error {
	switch req := r.(type) {
	case ScanRequest:
		c.startScan()
		return nil
	case RefreshRequest:
		return c.refresh(req.Paths)
	case AddTagRequest:
		return c.addTag(req.Paths, req.Tag)
	case RemoveTagRequest:
		return c.removeTag(req.Paths, req.Tag)
	case SetTagOrderRequest:
		return c.store.SetTagOrder(req.Order)
	case SetAnnotationRequest:
		return c.setAnnotation(req.Paths, req.Key, req.Value)
	case AddAnnotationKeyRequest:
		_, err := c.store.AddAnnotationKey(req.Key)
		return err
	case SetLayoutRequest:
		if err := c.store.SetLayout(req.Layout); err != nil {
			return err
		}
		c.emit(PrefsChanged{})
		return nil
	case SetPathRelativeRequest:
		if err := c.store.SetPathRelative(req.Relative); err != nil {
			return err
		}
		c.emit(PrefsChanged{})
		return nil
	case SetPathTildeRequest:
		if err := c.store.SetPathTilde(req.Tilde); err != nil {
			return err
		}
		c.emit(PrefsChanged{})
		return nil
	case SetTableModeRequest:
		if err := c.store.SetTableMode(req.Enabled); err != nil {
			return err
		}
		c.emit(PrefsChanged{})
		return nil
	case SetSessionRequest:
		return c.store.SetSession(c.root, req.Session)
	case SaveViewRequest:
		return c.store.SaveView(c.root, req.Name, req.Config)
	case SetRootRequest:
		return c.setRoot(req.Root)
	default:
		return fmt.Errorf("engine: unknown request %T", r)
	}
}

func (c *Core) setRoot(root string) error {
	normalized, err := project.NormalizeRoot(root)
	if err != nil {
		return fmt.Errorf("root: %w", err)
	}
	fi, err := os.Stat(normalized)
	if err != nil {
		return fmt.Errorf("root: %w", err)
	}
	if !fi.IsDir() {
		return fmt.Errorf("root is not a directory: %s", normalized)
	}
	if err := c.store.RememberRoot(normalized); err != nil {
		return err
	}

	c.mu.Lock()
	if c.root == normalized {
		c.mu.Unlock()
		return nil
	}
	c.scanGen++
	c.root = normalized
	c.all = nil
	c.found = 0
	c.mu.Unlock()

	c.emit(RootChanged{Root: normalized})
	c.loadCachedProjects()
	c.startScan()
	return nil
}

func (c *Core) startScan() <-chan struct{} {
	done := make(chan struct{})
	c.mu.Lock()
	// A newer scan supersedes one already walking the tree.
	c.scanGen++
	c.scanOn = true
	gen := c.scanGen
	root := c.root
	c.mu.Unlock()
	c.emit(ScanStarted{})
	st := c.store
	c.scanWG.Add(1)
	go func() {
		defer close(done)
		defer c.scanWG.Done()
		err := project.Scan(root, func(f project.Found) {
			if c.ctx.Err() != nil {
				return
			}
			c.mu.Lock()
			stale := gen != c.scanGen
			c.mu.Unlock()
			if stale {
				return
			}
			st.NoteGitProject(f.Path)
			p := Project{
				Path:        f.Path,
				Rel:         f.Rel,
				Tags:        st.TagsFor(f.Path),
				Meta:        map[string]any{},
				Annotations: st.AnnotationsFor(f.Path),
			}
			c.upsert(p)
			c.emit(ProjectFound{Project: p.clone()})
			slog.Info("project found, enriching helpers", "path", f.Path, "rel", f.Rel)
			c.helpers.Enrich(f.Path)
		})
		c.mu.Lock()
		stale := gen != c.scanGen
		c.mu.Unlock()
		if stale {
			return
		}
		if c.ctx.Err() != nil {
			c.mu.Lock()
			c.scanOn = false
			c.mu.Unlock()
			return
		}
		errStr := ""
		if err != nil {
			errStr = err.Error()
		}
		c.finishScan(gen, errStr)
	}()
	return done
}

func (c *Core) finishScan(gen uint64, errStr string) {
	c.mu.Lock()
	if gen != c.scanGen {
		c.mu.Unlock()
		return
	}
	c.scanOn = false
	c.pruneLocked()
	c.found = len(c.all)
	root := c.root
	c.mu.Unlock()
	_ = c.store.PersistProjectsAndSettings()
	c.persistCacheRoot(root)
	c.emit(ScanFinished{Err: errStr})
}

func (c *Core) loadCachedProjects() {
	cached := c.store.CachedRoot(c.root)
	valid := make([]store.CachedProject, 0, len(cached))
	for _, entry := range cached {
		if !project.IsIndexed(entry.Path) {
			_ = c.store.RemoveProject(entry.Path)
			continue
		}
		valid = append(valid, entry)
		tags := c.store.TagsFor(entry.Path)
		for _, tag := range tags {
			_ = c.store.EnsureTagColor(tag)
		}
		p := Project{
			Path:        entry.Path,
			Rel:         project.RepoName(entry.Path),
			Tags:        tags,
			Meta:        map[string]any{},
			Annotations: c.store.AnnotationsFor(entry.Path),
		}
		c.upsert(p)
		slog.Info("cached project loaded, enriching helpers", "path", entry.Path, "rel", p.Rel)
		c.helpers.Enrich(entry.Path)
	}
	if len(valid) != len(cached) {
		_ = c.store.SetCacheRoot(c.root, valid)
	}
	c.mu.Lock()
	c.pruneLocked()
	c.found = len(c.all)
	c.mu.Unlock()
}

func (c *Core) drainHelpers() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.helperEvents:
			if !ok {
				return
			}
			switch m := msg.(type) {
			case helpers.MetaMsg:
				switch {
				case m.Err != nil:
					slog.Warn("project helper result ignored",
						"path", m.Path,
						"helper", m.Helper,
						"reason", "error",
						"err", m.Err,
					)
				case len(m.Meta) == 0:
					slog.Info("project helper result ignored",
						"path", m.Path,
						"helper", m.Helper,
						"reason", "empty metadata",
					)
				default:
					slog.Debug("project helper result applied",
						"path", m.Path,
						"helper", m.Helper,
						"meta_keys", len(m.Meta),
					)
					c.applyMeta(m.Path, m.Helper, m.Meta)
				}
			case helpers.GitHubPendingMsg:
				c.emit(GitHubPending{Count: c.githubPending.Count()})
			}
		}
	}
}

func (c *Core) applyMeta(path, helper string, meta map[string]any) {
	tags, rest := helpers.SplitTags(meta)
	if helper == "git" {
		if _, ok := meta[helpers.MetaKeyTags]; ok {
			c.syncGitStatusTags(path, tags)
		}
	} else {
		for _, tag := range tags {
			_, _ = c.store.AddTag(path, tag)
		}
	}
	c.mu.Lock()
	idx := -1
	for i := range c.all {
		if c.all[i].Path == path {
			idx = i
			break
		}
	}
	if idx < 0 {
		c.mu.Unlock()
		return
	}
	c.all[idx].Meta = mergeMeta(c.all[idx].Meta, rest)
	if len(tags) > 0 {
		c.all[idx].Tags = mergeTags(c.all[idx].Tags, c.store.TagsFor(path))
	}
	updated := c.all[idx].clone()
	c.mu.Unlock()
	c.emit(ProjectUpdated{Project: updated})
}

func (c *Core) syncGitStatusTags(path string, want []string) {
	wantSet := map[string]bool{}
	for _, tag := range want {
		tag = store.NormalizeTag(tag)
		if tag != "" {
			wantSet[tag] = true
		}
	}
	for _, name := range helpers.GitStatusTagNames {
		if wantSet[name] {
			_, _ = c.store.AddTag(path, name)
		} else {
			_, _ = c.store.RemoveTag(path, name)
		}
	}
}

func (c *Core) upsert(p Project) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p.Meta == nil {
		p.Meta = map[string]any{}
	}
	if p.Annotations == nil {
		p.Annotations = c.store.AnnotationsFor(p.Path)
	}
	for i := range c.all {
		if c.all[i].Path != p.Path {
			continue
		}
		p.Tags = mergeTags(c.all[i].Tags, p.Tags)
		p.Meta = mergeMeta(c.all[i].Meta, p.Meta)
		c.all[i] = p
		c.found = len(c.all)
		return
	}
	c.all = append(c.all, p)
	c.found = len(c.all)
}

func (c *Core) projectByPath(path string) (Project, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.all {
		if p.Path == path {
			return p.clone(), true
		}
	}
	return Project{}, false
}

func (c *Core) refresh(paths []string) error {
	icons := helpers.NewIconHelper(c.store.CacheDir())
	for _, path := range paths {
		slog.Info("refreshing project helpers", "path", path)
		if meta, err := icons.CollectForced(c.ctx, path); err == nil && meta != nil {
			c.applyMeta(path, icons.Name(), meta)
		} else if err != nil {
			slog.Warn("icon helper refresh failed", "path", path, "err", err)
		}
		c.helpers.Refresh(path)
	}
	return nil
}

// DeleteLocalProject removes the project's directory from the local disk and
// rescans the root. It does not contact GitHub or any other remote. Deletion
// is refused when the project has the "changes" tag.
func (c *Core) DeleteLocalProject(ctx context.Context, path string) error {
	p, ok := c.projectByPath(path)
	if !ok {
		return ErrProjectNotFound
	}
	for _, tag := range p.Tags {
		if store.NormalizeTag(tag) == helpers.GitTagChanges {
			return ErrLocalChanges
		}
	}
	c.mu.Lock()
	root := c.root
	c.mu.Unlock()
	dir, err := project.DirInsideRoot(root, p.Path)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return c.Rescan(ctx)
}

func (c *Core) addTag(paths []string, tag string) error {
	if helpers.IsGitStatusTag(tag) {
		return ErrReservedTag
	}
	var err error
	for _, path := range paths {
		if _, addErr := c.store.AddTag(path, tag); addErr != nil {
			err = addErr
			continue
		}
		c.reloadTags(path)
	}
	return err
}

func (c *Core) removeTag(paths []string, tag string) error {
	if helpers.IsGitStatusTag(tag) {
		return ErrReservedTag
	}
	var err error
	for _, path := range paths {
		if _, remErr := c.store.RemoveTag(path, tag); remErr != nil {
			err = remErr
			continue
		}
		c.reloadTags(path)
	}
	return err
}

func (c *Core) reloadTags(path string) {
	c.mu.Lock()
	for i := range c.all {
		if c.all[i].Path != path {
			continue
		}
		c.all[i].Tags = c.store.TagsFor(path)
		updated := c.all[i].clone()
		c.mu.Unlock()
		c.emit(ProjectUpdated{Project: updated})
		return
	}
	c.mu.Unlock()
}

func (c *Core) setAnnotation(paths []string, key, value string) error {
	var err error
	for _, path := range paths {
		if setErr := c.store.SetAnnotation(path, key, value); setErr != nil {
			err = setErr
			continue
		}
		c.mu.Lock()
		for i := range c.all {
			if c.all[i].Path != path {
				continue
			}
			c.all[i].Annotations = c.store.AnnotationsFor(path)
			updated := c.all[i].clone()
			c.mu.Unlock()
			c.emit(ProjectUpdated{Project: updated})
			goto next
		}
		c.mu.Unlock()
	next:
	}
	return err
}

func (c *Core) persistCache() {
	c.persistCacheRoot(c.root)
}

func (c *Core) persistCacheRoot(root string) {
	c.mu.Lock()
	c.pruneLocked()
	cached := make([]store.CachedProject, 0, len(c.all))
	for _, p := range c.all {
		if !project.IsIndexed(p.Path) {
			continue
		}
		cached = append(cached, store.CachedProject{Path: p.Path, Rel: p.Rel})
	}
	c.mu.Unlock()
	_ = c.store.SetCacheRoot(root, cached)
}

// pruneLocked drops deleted repos. Caller must hold c.mu.
func (c *Core) pruneLocked() {
	removed := make([]string, 0)
	kept := make([]Project, 0, len(c.all))
	for _, p := range c.all {
		if project.IsIndexed(p.Path) {
			kept = append(kept, p)
			continue
		}
		_ = c.store.RemoveProject(p.Path)
		removed = append(removed, p.Path)
	}
	c.all = kept
	c.found = len(c.all)
	if len(removed) == 0 {
		return
	}
	paths := append([]string{}, removed...)
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		for _, path := range paths {
			c.emit(ProjectRemoved{Path: path})
		}
	}()
}

func mergeTags(a, b []string) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, set := range [][]string{a, b} {
		for _, tag := range set {
			if _, ok := seen[tag]; ok {
				continue
			}
			seen[tag] = struct{}{}
			out = append(out, tag)
		}
	}
	return out
}

func mergeMeta(a, b map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		if s, ok := v.(string); ok && s == "" {
			delete(out, k)
			continue
		}
		out[k] = v
	}
	return out
}
