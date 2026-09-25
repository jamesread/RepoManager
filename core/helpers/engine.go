// Package helpers enriches discovered projects with metadata in the
// background. Each Helper (git, GitHub, modules, mtime, icon) declares the Fields
// it produces and collects values for one project at a time; the Engine
// bounds parallelism per helper and reports results on an events channel as
// MetaMsg values.
package helpers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const helperMaxParallel = 5

// FieldKind describes how a helper metadata field can be sorted/filtered.
type FieldKind string

const (
	FieldString FieldKind = "string"
	FieldTime   FieldKind = "time"
	FieldInt    FieldKind = "int"
	FieldMulti  FieldKind = "multi" // e.g. topic lists
)

// Field is a metadata attribute that helpers may attach to a project.
type Field struct {
	ID         string    // stable id, e.g. "git.last_commit"
	Label      string    // UI label, e.g. "Git updated"
	Kind       FieldKind // drives sort/filter behaviour
	Sortable   bool
	Filterable bool
}

// MetaKeyTags is a reserved key in a helper's result map. Its value
// ([]string) lists tags the helper wants registered on the project. The
// engine strips it from the metadata before it is merged into the project.
const MetaKeyTags = "_tags"

// normalizeTag trims surrounding whitespace from a tag name.
func normalizeTag(tag string) string { return strings.TrimSpace(tag) }

// SplitTags separates helper-requested tags from ordinary metadata.
// The returned map is a shallow copy without the reserved key.
func SplitTags(meta map[string]any) (tags []string, rest map[string]any) {
	rest = make(map[string]any, len(meta))
	for k, v := range meta {
		if k != MetaKeyTags {
			rest[k] = v
			continue
		}
		switch t := v.(type) {
		case []string:
			tags = append(tags, t...)
		case string:
			if t = normalizeTag(t); t != "" {
				tags = append(tags, t)
			}
		}
	}
	return tags, rest
}

// Helper enriches a single project with metadata in the background.
type Helper interface {
	Name() string
	Fields() []Field
	Collect(ctx context.Context, projectPath string) (map[string]any, error)
}

// ForceHelper can bypass caches when the user requests a manual refresh.
type ForceHelper interface {
	Helper
	CollectForced(ctx context.Context, projectPath string) (map[string]any, error)
}

// MetaMsg is delivered on the events channel when a helper finishes a
// project. Err is set when collection failed; Meta may then be empty.
type MetaMsg struct {
	Path   string
	Helper string
	Meta   map[string]any
	Err    error
}

type helperSlot struct {
	helper Helper
	sem    chan struct{} // capacity = helperMaxParallel
}

// Engine runs registered helpers with a per-helper concurrency cap.
type Engine struct {
	slots  []helperSlot
	events chan<- any
	ctx    context.Context
	wg     sync.WaitGroup
}

func Default(cacheDir string, githubTTL time.Duration, ghPending *GitHubQueryState, onGitHubPendingChange func()) []Helper {
	return []Helper{
		MtimeHelper{},
		ModulesHelper{},
		NewIconHelper(cacheDir),
		GitHelper{},
		NewGitHubHelper(cacheDir, githubTTL, ghPending, onGitHubPendingChange),
	}
}

func NewEngine(ctx context.Context, events chan<- any, helpers []Helper) *Engine {
	slots := make([]helperSlot, 0, len(helpers))
	for _, h := range helpers {
		slots = append(slots, helperSlot{
			helper: h,
			sem:    make(chan struct{}, helperMaxParallel),
		})
	}
	return &Engine{slots: slots, events: events, ctx: ctx}
}

// HelperCount is how many helpers run per project (results expected per Enrich/Refresh).
func (e *Engine) HelperCount() int {
	if e == nil {
		return 0
	}
	return len(e.slots)
}

// Fields returns every field registered by helpers (plus the built-in name field).
func (e *Engine) Fields() []Field {
	fields := []Field{{
		ID:         "name",
		Label:      "Name",
		Kind:       FieldString,
		Sortable:   true,
		Filterable: true,
	}, {
		ID:         "tags",
		Label:      "Tags",
		Kind:       FieldMulti,
		Sortable:   false,
		Filterable: true,
	}}
	for _, slot := range e.slots {
		fields = append(fields, slot.helper.Fields()...)
	}
	return fields
}

func (e *Engine) SortableFields() []Field {
	var out []Field
	for _, f := range e.Fields() {
		if f.Sortable {
			out = append(out, f)
		}
	}
	return out
}

func (e *Engine) FilterableFields() []Field {
	var out []Field
	for _, f := range e.Fields() {
		if f.Filterable {
			out = append(out, f)
		}
	}
	return out
}

func (e *Engine) FieldByID(id string) (Field, bool) {
	for _, f := range e.Fields() {
		if f.ID == id {
			return f, true
		}
	}
	return Field{}, false
}

// Enrich schedules all helpers for a project path (non-blocking).
func (e *Engine) Enrich(projectPath string) {
	e.schedule(projectPath, false)
}

// Refresh force-refetches helpers for a project (bypasses helper caches).
func (e *Engine) Refresh(projectPath string) {
	e.schedule(projectPath, true)
}

func (e *Engine) schedule(projectPath string, force bool) {
	if e == nil || projectPath == "" {
		return
	}
	for i := range e.slots {
		slot := e.slots[i]
		e.wg.Add(1)
		go e.runOne(slot, projectPath, force)
	}
}

func (e *Engine) runOne(slot helperSlot, projectPath string, force bool) {
	defer e.wg.Done()
	select {
	case slot.sem <- struct{}{}:
	case <-e.ctx.Done():
		return
	}
	defer func() { <-slot.sem }()

	var (
		meta map[string]any
		err  error
	)
	if force {
		if fh, ok := slot.helper.(ForceHelper); ok {
			meta, err = fh.CollectForced(e.ctx, projectPath)
		} else {
			meta, err = slot.helper.Collect(e.ctx, projectPath)
		}
	} else {
		meta, err = slot.helper.Collect(e.ctx, projectPath)
	}
	if e.ctx.Err() != nil {
		return
	}
	msg := MetaMsg{
		Path:   projectPath,
		Helper: slot.helper.Name(),
		Meta:   meta,
		Err:    err,
	}
	select {
	case e.events <- msg:
	case <-e.ctx.Done():
	}
}

// Wait blocks until all in-flight Enrich/Refresh helpers have finished.
func (e *Engine) Wait() {
	if e == nil {
		return
	}
	e.wg.Wait()
}

func FormatMetaValue(v any) string {
	switch t := v.(type) {
	case time.Time:
		if t.IsZero() {
			return ""
		}
		return RelativeTime(t)
	case string:
		return t
	case []string:
		return strings.Join(t, ", ")
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func MetaValuesForBrowse(field Field, v any) []string {
	if v == nil {
		return nil
	}
	switch field.Kind {
	case FieldMulti:
		switch t := v.(type) {
		case []string:
			return append([]string{}, t...)
		case string:
			parts := strings.Split(t, ",")
			out := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					out = append(out, p)
				}
			}
			return out
		}
	}
	if s := FormatMetaValue(v); s != "" {
		return []string{s}
	}
	return nil
}

func RelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 365*24*time.Hour:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	default:
		return fmt.Sprintf("%dy ago", int(d.Hours()/(24*365)))
	}
}

func CompareMeta(kind FieldKind, a, b any) int {
	aMissing := a == nil
	bMissing := b == nil
	if aMissing && bMissing {
		return 0
	}
	if aMissing {
		return 1 // missing sorts last
	}
	if bMissing {
		return -1
	}

	switch kind {
	case FieldTime:
		ta, oka := a.(time.Time)
		tb, okb := b.(time.Time)
		if !oka || !okb {
			return 0
		}
		switch {
		case ta.After(tb):
			return -1 // newer first
		case ta.Before(tb):
			return 1
		default:
			return 0
		}
	case FieldInt:
		ia := toInt64(a)
		ib := toInt64(b)
		switch {
		case ia < ib:
			return -1
		case ia > ib:
			return 1
		default:
			return 0
		}
	default:
		sa := strings.ToLower(fmt.Sprintf("%v", a))
		sb := strings.ToLower(fmt.Sprintf("%v", b))
		switch {
		case sa < sb:
			return -1
		case sa > sb:
			return 1
		default:
			return 0
		}
	}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}
