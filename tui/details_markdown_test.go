package main

import (
	"strings"
	"testing"
	"time"

	"github.com/jamesread/RepoManager/core/helpers"
)

func TestProjectDetailsMarkdown(t *testing.T) {
	st := newTestStore(t)
	_, _ = st.AddAnnotationKey("status")

	m := &model{
		store: st,
	}
	attachTestEngine(t, m, st, t.TempDir())

	when := time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local)
	p := projectItem{
		path: "/home/user/sandbox/myapp",
		rel:  "myapp",
		tags: []string{"Go", "Git"},
		annotations: map[string]string{
			"status": "active",
		},
		meta: map[string]any{
			helpers.FieldDirMtime: when,
		},
	}

	out := m.projectDetailsMarkdown(p)
	if !strings.HasPrefix(out, "# myapp\n\n") {
		t.Fatalf("header:\n%s", out)
	}
	if !strings.Contains(out, "`/home/user/sandbox/myapp`\n\n") {
		t.Fatalf("path missing:\n%s", out)
	}
	if !strings.Contains(out, "## Tags\n\n") || !strings.Contains(out, "- Git\n") || !strings.Contains(out, "- Go\n") {
		t.Fatalf("tags missing:\n%s", out)
	}
	if !strings.Contains(out, "**@status**: `active`\n") {
		t.Fatalf("annotations missing:\n%s", out)
	}
	if !strings.Contains(out, "## Metadata\n\n") {
		t.Fatalf("metadata section missing:\n%s", out)
	}
}
