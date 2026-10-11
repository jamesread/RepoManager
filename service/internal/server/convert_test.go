package server

import (
	"testing"
	"time"

	"github.com/jamesread/RepoManager/core/engine"
)

func TestProjectToProtoMeta(t *testing.T) {
	when := time.Date(2024, 3, 15, 12, 0, 0, 0, time.UTC)
	p := engine.Project{
		Path: "/tmp/repo",
		Rel:  "repo",
		Meta: map[string]any{
			"dir.mtime":           when,
			"github.owner":        "acme",
			"github.topics":       []string{"go", "tui"},
			"github.open_issues":  3,
			"git.last_commit":     when,
			"modules":             []string{"Go"},
			"icon":                "logo.png",
			"icon.rev":            "abc123",
			"empty.string":        "",
			"unsupported":         struct{ x int }{x: 1},
		},
	}
	out := projectToProto(p)
	if out.Meta == nil || len(out.Meta.Fields) == 0 {
		t.Fatalf("meta fields empty: %#v", out.Meta)
	}
	if got := out.Meta.Fields["github.owner"].GetStringValue(); got != "acme" {
		t.Fatalf("github.owner = %q", got)
	}
	if got := out.Meta.Fields["dir.mtime"].GetStringValue(); got != when.Format(time.RFC3339Nano) {
		t.Fatalf("dir.mtime = %q", got)
	}
	topics := out.Meta.Fields["github.topics"].GetListValue().GetValues()
	if len(topics) != 2 || topics[0].GetStringValue() != "go" {
		t.Fatalf("github.topics = %#v", topics)
	}
	if got := out.Meta.Fields["github.open_issues"].GetNumberValue(); got != 3 {
		t.Fatalf("github.open_issues = %v", got)
	}
	if _, ok := out.Meta.Fields["empty.string"]; ok {
		t.Fatal("empty string should be omitted")
	}
	if got := out.Meta.Fields["unsupported"].GetStringValue(); got != "{1}" {
		t.Fatalf("unsupported = %q", got)
	}
}

func TestMetaValueToProto(t *testing.T) {
	when := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	v, ok := metaValueToProto(when)
	if !ok {
		t.Fatal("time.Time should convert")
	}
	if s, ok := v.(string); !ok || s != when.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("time = %#v", v)
	}
	v, ok = metaValueToProto([]string{"a"})
	if !ok {
		t.Fatal("[]string should convert")
	}
	list, ok := v.([]any)
	if !ok || len(list) != 1 || list[0] != "a" {
		t.Fatalf("[]string = %#v", v)
	}
}
