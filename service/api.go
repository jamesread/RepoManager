package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
)

type fieldDTO struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Kind       string `json:"kind"`
	Sortable   bool   `json:"sortable"`
	Filterable bool   `json:"filterable"`
}

type projectsResponse struct {
	Root        string            `json:"root"`
	Scanning    bool              `json:"scanning"`
	Found       int               `json:"found"`
	Fields      []fieldDTO        `json:"fields"`
	Projects    []engine.Project  `json:"projects"`
	RecentRoots []string          `json:"recent_roots"`
	KnownTags   []string          `json:"known_tags"`
	TagColors   map[string]string `json:"tag_colors"`
}

type projectResponse struct {
	Root      string            `json:"root"`
	Fields    []fieldDTO        `json:"fields"`
	Project   engine.Project    `json:"project"`
	KnownTags []string          `json:"known_tags"`
	TagColors map[string]string `json:"tag_colors"`
}

func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.projectsBody())
}

func (s *Server) handleProject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	p, ok := findProject(s.eng.Snapshot(), path)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	writeJSON(w, projectResponse{
		Root:      s.eng.Root(),
		Fields:    catalogFields(s.eng),
		Project:   p,
		KnownTags: knownTags(s.eng),
		TagColors: s.tagColors(),
	})
}

func (s *Server) handleProjectIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	p, ok := findProject(s.eng.Snapshot(), path)
	if !ok || projectIcon(p) == "" {
		http.NotFound(w, r)
		return
	}
	f, ctype, err := helpers.OpenCachedIcon(s.eng.Store().CacheDir(), path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "icon unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=60")
	http.ServeContent(w, r, projectIcon(p), info.ModTime(), f)
}

func projectIcon(p engine.Project) string {
	if p.Meta == nil {
		return ""
	}
	icon, _ := p.Meta[helpers.FieldIcon].(string)
	return strings.TrimSpace(icon)
}

func (s *Server) handleProjectRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path  string   `json:"path"`
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	paths := refreshPaths(body.Path, body.Paths)
	if len(paths) == 0 {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	if !projectsContain(s.eng.Snapshot(), paths) {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if err := s.eng.Send(r.Context(), engine.RefreshRequest{Paths: paths}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func refreshPaths(path string, paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths)+1)
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(path)
	for _, p := range paths {
		add(p)
	}
	return out
}

func (s *Server) handleProjectDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Path = strings.TrimSpace(body.Path)
	if body.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	if err := s.eng.DeleteLocalProject(r.Context(), body.Path); err != nil {
		switch {
		case errors.Is(err, engine.ErrProjectNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, engine.ErrLocalChanges):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
		}
		return
	}
	writeJSON(w, s.projectsBody())
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.eng.Rescan(r.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.projectsBody())
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Root string `json:"root"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(body.Root) == "" {
		http.Error(w, "root is required", http.StatusBadRequest)
		return
	}
	if err := s.eng.Send(r.Context(), engine.SetRootRequest{Root: body.Root}); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, s.projectsBody())
}

func (s *Server) projectsBody() projectsResponse {
	return projectsResponse{
		Root:        s.eng.Root(),
		Scanning:    s.eng.Scanning(),
		Found:       s.eng.Found(),
		Fields:      catalogFields(s.eng),
		Projects:    s.eng.Snapshot(),
		RecentRoots: s.eng.Store().RecentRoots(),
		KnownTags:   knownTags(s.eng),
		TagColors:   s.tagColors(),
	}
}

func (s *Server) tagColors() map[string]string {
	colors := s.eng.Store().EnsureTagPastels(knownTags(s.eng))
	if colors == nil {
		return map[string]string{}
	}
	return colors
}

func knownTags(eng *engine.Core) []string {
	tags := eng.Store().AllKnownTags()
	if tags == nil {
		return []string{}
	}
	sort.Strings(tags)
	return tags
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Paths []string `json:"paths"`
		Tag   string   `json:"tag"`
		Op    string   `json:"op"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	req, status, msg := tagMutation(s.eng.Snapshot(), body.Paths, body.Tag, body.Op)
	if status != 0 {
		http.Error(w, msg, status)
		return
	}
	if err := s.eng.Send(r.Context(), req); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, s.projectsBody())
}

func (s *Server) handleAnnotations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Path  string `json:"path"`
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	body.Path = strings.TrimSpace(body.Path)
	body.Key = strings.TrimSpace(body.Key)
	if body.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}
	if body.Key == "" {
		http.Error(w, "key is required", http.StatusBadRequest)
		return
	}
	if _, ok := findProject(s.eng.Snapshot(), body.Path); !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if err := s.eng.Send(r.Context(), engine.SetAnnotationRequest{
		Paths: []string{body.Path},
		Key:   body.Key,
		Value: body.Value,
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	p, ok := findProject(s.eng.Snapshot(), body.Path)
	if !ok {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	writeJSON(w, projectResponse{
		Root:      s.eng.Root(),
		Fields:    catalogFields(s.eng),
		Project:   p,
		KnownTags: knownTags(s.eng),
		TagColors: s.tagColors(),
	})
}

func tagMutation(projects []engine.Project, paths []string, tag, op string) (engine.Request, int, string) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return nil, http.StatusBadRequest, "tag is required"
	}
	if helpers.IsGitStatusTag(tag) {
		return nil, http.StatusBadRequest, "tag is set by a helper"
	}
	if len(paths) == 0 {
		return nil, http.StatusBadRequest, "paths are required"
	}
	if !projectsContain(projects, paths) {
		return nil, http.StatusNotFound, "project not found"
	}
	switch op {
	case "add":
		return engine.AddTagRequest{Paths: paths, Tag: tag}, 0, ""
	case "remove":
		return engine.RemoveTagRequest{Paths: paths, Tag: tag}, 0, ""
	default:
		return nil, http.StatusBadRequest, "op must be add or remove"
	}
}

func projectsContain(projects []engine.Project, paths []string) bool {
	known := make(map[string]struct{}, len(projects))
	for _, p := range projects {
		known[p.Path] = struct{}{}
	}
	for _, path := range paths {
		if _, ok := known[path]; !ok {
			return false
		}
	}
	return true
}

func findProject(projects []engine.Project, path string) (engine.Project, bool) {
	for _, p := range projects {
		if p.Path == path {
			return p, true
		}
	}
	return engine.Project{}, false
}

func catalogFields(eng *engine.Core) []fieldDTO {
	fields := eng.Fields()
	out := make([]fieldDTO, 0, len(fields)+len(eng.Store().AnnotationKeys()))
	for _, f := range fields {
		out = append(out, toFieldDTO(f))
	}
	for _, key := range eng.Store().AnnotationKeys() {
		out = append(out, fieldDTO{
			ID:         "annotation." + key,
			Label:      "@" + key,
			Kind:       string(helpers.FieldString),
			Sortable:   true,
			Filterable: true,
		})
	}
	return out
}

func toFieldDTO(f helpers.Field) fieldDTO {
	return fieldDTO{
		ID:         f.ID,
		Label:      f.Label,
		Kind:       string(f.Kind),
		Sortable:   f.Sortable,
		Filterable: f.Filterable,
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}
