package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/helpers"
	"github.com/jamesread/RepoManager/core/store"
	"github.com/stretchr/testify/require"
)

func TestHandleProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()

	s := NewForTest(eng)
	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body projectsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, root, body.Root)
	require.NotNil(t, body.Projects)
	require.NotEmpty(t, body.Fields)
	ids := make(map[string]bool, len(body.Fields))
	for _, f := range body.Fields {
		ids[f.ID] = true
	}
	require.True(t, ids["name"])
	require.True(t, ids["tags"])
	require.True(t, ids["dir.mtime"])
	require.True(t, ids["git.last_commit"])
}

func TestHandleProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	projectDir := filepath.Join(root, "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		for _, p := range eng.Snapshot() {
			if p.Path == projectDir {
				return true
			}
		}
		return false
	}, time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	req := httptest.NewRequest(http.MethodGet, "/api/project?path="+url.QueryEscape(projectDir), nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body projectResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, projectDir, body.Project.Path)
	require.Equal(t, "demo", body.Project.Rel)
}

func TestHandleProjectIcon(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	projectDir := filepath.Join(root, "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755))
	logo := []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	require.NoError(t, os.WriteFile(filepath.Join(projectDir, "logo.svg"), logo, 0o644))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		for _, p := range eng.Snapshot() {
			if p.Path == projectDir && p.Meta["icon"] == "logo.svg" {
				return true
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	req := httptest.NewRequest(http.MethodGet, "/api/project/icon?path="+url.QueryEscape(projectDir), nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	require.Equal(t, string(logo), rec.Body.String())

	missing := httptest.NewRequest(http.MethodGet, "/api/project/icon?path="+url.QueryEscape(filepath.Join(root, "missing")), nil)
	missRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(missRec, missing)
	require.Equal(t, http.StatusNotFound, missRec.Code)
}

func TestHandleProjectNotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	eng := engine.New(st, t.TempDir())
	defer eng.Close()
	_ = eng.Subscribe()

	s := NewForTest(eng)
	req := httptest.NewRequest(http.MethodGet, "/api/project?path=/missing", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestHandleProjectRefresh(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	projectDir := filepath.Join(root, "demo")
	otherDir := filepath.Join(root, "other")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(otherDir, ".git"), 0o755))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		seen := map[string]bool{}
		for _, p := range eng.Snapshot() {
			seen[p.Path] = true
		}
		return seen[projectDir] && seen[otherDir]
	}, time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	body := strings.NewReader(`{"path":` + strconv.Quote(projectDir) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/project/refresh", body)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	bulk, err := json.Marshal(map[string]any{"paths": []string{projectDir, otherDir}})
	require.NoError(t, err)
	req = httptest.NewRequest(http.MethodPost, "/api/project/refresh", strings.NewReader(string(bulk)))
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestHandleRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	rootA := t.TempDir()
	rootB := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(rootB, "demo", ".git"), 0o755))

	eng := engine.New(st, rootA)
	defer eng.Close()
	_ = eng.Subscribe()

	s := NewForTest(eng)
	body := strings.NewReader(`{"root":` + strconv.Quote(rootB) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/root", body)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp projectsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	require.Equal(t, rootB, resp.Root)
	require.Equal(t, rootB, eng.Root())
	require.NotEmpty(t, resp.RecentRoots)
	require.Equal(t, resp.Root, resp.RecentRoots[0])
}

func TestHandleRootMissing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	eng := engine.New(st, t.TempDir())
	defer eng.Close()
	_ = eng.Subscribe()

	s := NewForTest(eng)
	missing := filepath.Join(t.TempDir(), "nope")
	body := strings.NewReader(`{"root":` + strconv.Quote(missing) + `}`)
	req := httptest.NewRequest(http.MethodPost, "/api/root", body)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestHandleTagsBulk(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	alpha := filepath.Join(root, "alpha")
	beta := filepath.Join(root, "beta")
	require.NoError(t, os.MkdirAll(filepath.Join(alpha, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(beta, ".git"), 0o755))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		seen := map[string]bool{}
		for _, p := range eng.Snapshot() {
			seen[p.Path] = true
		}
		return seen[alpha] && seen[beta]
	}, time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	post := func(op, tag string, paths ...string) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(map[string]any{
			"op":    op,
			"tag":   tag,
			"paths": paths,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/tags", strings.NewReader(string(payload)))
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}

	rec := post("add", "wip", alpha, beta)
	require.Equal(t, http.StatusOK, rec.Code)
	var body projectsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Contains(t, tagsFor(body.Projects, alpha), "wip")
	require.Contains(t, tagsFor(body.Projects, beta), "wip")
	require.Contains(t, body.KnownTags, "wip")
	require.True(t, isHexColor(body.TagColors["wip"]))

	rec = post("remove", "wip", alpha)
	require.Equal(t, http.StatusOK, rec.Code)
	body = projectsResponse{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.NotContains(t, tagsFor(body.Projects, alpha), "wip")
	require.Contains(t, tagsFor(body.Projects, beta), "wip")

	require.Equal(t, http.StatusBadRequest, post("nope", "wip", beta).Code)
	require.Equal(t, http.StatusNotFound, post("add", "wip", filepath.Join(root, "missing")).Code)

	for _, tag := range []string{"changes", "behind", "ahead", "Changes"} {
		require.Equal(t, http.StatusBadRequest, post("add", tag, alpha).Code)
		require.Equal(t, http.StatusBadRequest, post("remove", tag, alpha).Code)
		require.ErrorIs(t, eng.Send(t.Context(), engine.AddTagRequest{Paths: []string{alpha}, Tag: tag}), engine.ErrReservedTag)
		require.ErrorIs(t, eng.Send(t.Context(), engine.RemoveTagRequest{Paths: []string{alpha}, Tag: tag}), engine.ErrReservedTag)
	}
}

func TestHandleAnnotations(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	projectDir := filepath.Join(root, "demo")
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, ".git"), 0o755))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		for _, p := range eng.Snapshot() {
			if p.Path == projectDir {
				return true
			}
		}
		return false
	}, time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	post := func(key, value string) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(map[string]any{
			"path":  projectDir,
			"key":   key,
			"value": value,
		})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/annotations", strings.NewReader(string(payload)))
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}

	rec := post("motivation", "ship it")
	require.Equal(t, http.StatusOK, rec.Code)
	var body projectResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Equal(t, "ship it", body.Project.Annotations["motivation"])
	require.Contains(t, body.Fields, fieldDTO{
		ID:         "annotation.motivation",
		Label:      "@motivation",
		Kind:       "string",
		Sortable:   true,
		Filterable: true,
	})

	rec = post("motivation", "  ")
	require.Equal(t, http.StatusOK, rec.Code)
	body = projectResponse{}
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.Empty(t, body.Project.Annotations["motivation"])
}

func TestHandleScanDropsDeletedProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	keepDir := filepath.Join(root, "keep")
	goneDir := filepath.Join(root, "gone")
	require.NoError(t, os.MkdirAll(filepath.Join(keepDir, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(goneDir, ".git"), 0o755))

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		found := map[string]bool{}
		for _, p := range eng.Snapshot() {
			found[p.Path] = true
		}
		return found[keepDir] && found[goneDir]
	}, 3*time.Second, 20*time.Millisecond)

	require.NoError(t, os.RemoveAll(goneDir))

	s := NewForTest(eng)
	req := httptest.NewRequest(http.MethodPost, "/api/scan", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var body projectsResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&body))
	require.False(t, body.Scanning)
	paths := make([]string, 0, len(body.Projects))
	for _, p := range body.Projects {
		paths = append(paths, p.Path)
	}
	require.Contains(t, paths, keepDir)
	require.NotContains(t, paths, goneDir)
}

func TestHandleProjectDelete(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))

	st, err := store.Load()
	require.NoError(t, err)

	root := t.TempDir()
	cleanDir := filepath.Join(root, "clean")
	dirtyDir := filepath.Join(root, "dirty")
	require.NoError(t, os.MkdirAll(filepath.Join(cleanDir, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dirtyDir, ".git"), 0o755))
	_, err = st.AddTag(dirtyDir, helpers.GitTagChanges)
	require.NoError(t, err)

	eng := engine.New(st, root)
	defer eng.Close()
	_ = eng.Subscribe()
	eng.Start()
	require.Eventually(t, func() bool {
		seen := map[string]bool{}
		for _, p := range eng.Snapshot() {
			seen[p.Path] = true
		}
		return seen[cleanDir] && seen[dirtyDir] && !eng.Scanning()
	}, 3*time.Second, 20*time.Millisecond)

	s := NewForTest(eng)
	post := func(path string) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(map[string]string{"path": path})
		require.NoError(t, err)
		req := httptest.NewRequest(http.MethodPost, "/api/project/delete", strings.NewReader(string(payload)))
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec
	}

	blocked := post(dirtyDir)
	require.Equal(t, http.StatusConflict, blocked.Code)
	_, err = os.Stat(dirtyDir)
	require.NoError(t, err)

	deleted := post(cleanDir)
	require.Equal(t, http.StatusOK, deleted.Code)
	_, err = os.Stat(cleanDir)
	require.True(t, os.IsNotExist(err))
	var body projectsResponse
	require.NoError(t, json.NewDecoder(deleted.Body).Decode(&body))
	for _, p := range body.Projects {
		require.NotEqual(t, cleanDir, p.Path)
	}
	require.False(t, body.Scanning)
}

func tagsFor(projects []engine.Project, path string) []string {
	for _, p := range projects {
		if p.Path == path {
			return p.Tags
		}
	}
	return nil
}

func isHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for _, c := range color[1:] {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
