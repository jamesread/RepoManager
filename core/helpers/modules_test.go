package helpers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDetectModules(t *testing.T) {
	dir := t.TempDir()
	touch := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	touch("go.mod")
	touch("package.json")
	touch("composer.json")
	touch("setup.py")
	touch("requirements.txt") // same label as setup.py; must dedupe
	// A directory named like a manifest must not count.
	if err := os.Mkdir(filepath.Join(dir, "Cargo.toml"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := detectModules(dir)
	want := "Go,PHP,Python,npm"
	if strings.Join(got, ",") != want {
		t.Fatalf("got %v want %s", got, want)
	}
}

func TestDetectModulesEmpty(t *testing.T) {
	if got := detectModules(t.TempDir()); len(got) != 0 {
		t.Fatalf("expected none, got %v", got)
	}
}

func TestModulesHelperRegistered(t *testing.T) {
	engine := NewEngine(context.Background(), make(chan any, 1), Default(t.TempDir(), time.Hour, nil, nil))
	if _, ok := engine.FieldByID(FieldModules); !ok {
		t.Fatal("modules field not registered")
	}
}
