package helpers

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFindProjectLogoPrefersSVG(t *testing.T) {
	dir := t.TempDir()
	writeLogo(t, filepath.Join(dir, "logo.png"), []byte("png"))
	writeLogo(t, filepath.Join(dir, "Logo.SVG"), []byte("<svg/>"))

	abs, _, ok := findProjectLogo(dir)
	if !ok || !strings.HasSuffix(strings.ToLower(abs), "logo.svg") {
		t.Fatalf("got %q ok=%v", abs, ok)
	}
}

func TestFindProjectLogoNested(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "frontend", "public")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogo(t, filepath.Join(nested, "logo.svg"), []byte("<svg/>"))

	abs, _, ok := findProjectLogo(dir)
	if !ok || !strings.HasSuffix(abs, filepath.Join("frontend", "public", "logo.svg")) {
		t.Fatalf("got %q ok=%v", abs, ok)
	}
}

func TestFindProjectLogoRootWins(t *testing.T) {
	dir := t.TempDir()
	writeLogo(t, filepath.Join(dir, "logo.png"), []byte("root"))
	nested := filepath.Join(dir, "public")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogo(t, filepath.Join(nested, "logo.svg"), []byte("<svg/>"))

	abs, _, ok := findProjectLogo(dir)
	if !ok || filepath.Base(abs) != "logo.png" {
		t.Fatalf("got %q ok=%v", abs, ok)
	}
}

func TestFindProjectLogoIgnoresNonFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "logo.svg"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "other.png")
	writeLogo(t, target, []byte("x"))
	if err := os.Symlink(target, filepath.Join(dir, "logo.png")); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := findProjectLogo(dir); ok {
		t.Fatal("expected no logo")
	}
}

func TestFindProjectLogoSkipsDependencies(t *testing.T) {
	dir := t.TempDir()
	vendor := filepath.Join(dir, "node_modules", "pkg")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogo(t, filepath.Join(vendor, "logo.svg"), []byte("<svg/>"))
	brand := filepath.Join(dir, "branding")
	if err := os.MkdirAll(brand, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLogo(t, filepath.Join(brand, "logo.png"), []byte("png"))

	abs, _, ok := findProjectLogo(dir)
	if !ok || filepath.Base(abs) != "logo.png" {
		t.Fatalf("got %q ok=%v", abs, ok)
	}
}

func TestFindProjectLogoTooLarge(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "logo.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxIconBytes + 1); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, _, ok := findProjectLogo(dir); ok {
		t.Fatal("expected oversized logo to be skipped")
	}
}

func TestIconHelperCaches(t *testing.T) {
	project := t.TempDir()
	cache := t.TempDir()
	src := filepath.Join(project, "logo.png")
	writeLogo(t, src, []byte("one"))

	h := NewIconHelper(cache)
	meta, err := h.Collect(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if meta[FieldIcon] != "logo.png" {
		t.Fatalf("meta = %#v", meta)
	}

	body, ctype := readCachedIcon(t, cache, project)
	if ctype != "image/png" || string(body) != "one" {
		t.Fatalf("cached %q %q", ctype, body)
	}

	// Unchanged source must not rewrite the cached bytes.
	if err := os.WriteFile(h.dataPath(project, ".png"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Collect(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	body, _ = readCachedIcon(t, cache, project)
	if string(body) != "stale" {
		t.Fatalf("cache rewritten, got %q", body)
	}

	if err := os.WriteFile(src, []byte("two!!"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Collect(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	body, _ = readCachedIcon(t, cache, project)
	if string(body) != "two!!" {
		t.Fatalf("got %q", body)
	}
}

func TestIconHelperForcedFlush(t *testing.T) {
	project := t.TempDir()
	cache := t.TempDir()
	src := filepath.Join(project, "logo.png")
	writeLogo(t, src, []byte("one"))

	h := NewIconHelper(cache)
	if _, err := h.Collect(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(h.dataPath(project, ".png"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	meta, err := h.CollectForced(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := readCachedIcon(t, cache, project)
	if string(body) != "one" {
		t.Fatalf("forced refresh left %q", body)
	}
	if meta[FieldIconRev] == "" {
		t.Fatal("missing icon rev")
	}

	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}
	meta, err = h.CollectForced(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	if meta[FieldIcon] != "" || meta[FieldIconRev] != "" {
		t.Fatalf("meta = %#v", meta)
	}
	if _, _, err := OpenCachedIcon(cache, project); err == nil {
		t.Fatal("cached logo was not removed")
	}
}

func TestIconHelperClearsMissing(t *testing.T) {
	meta, err := NewIconHelper(t.TempDir()).Collect(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if meta[FieldIcon] != "" {
		t.Fatalf("meta = %#v", meta)
	}
}

func TestIconHelperRegistered(t *testing.T) {
	engine := NewEngine(context.Background(), make(chan any, 1), Default(t.TempDir(), time.Hour, nil, nil))
	if _, ok := engine.FieldByID(FieldIcon); !ok {
		t.Fatal("icon field not registered")
	}
}

func writeLogo(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCachedIcon(t *testing.T, cache, project string) ([]byte, string) {
	t.Helper()
	f, ctype, err := OpenCachedIcon(cache, project)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	body, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return body, ctype
}
