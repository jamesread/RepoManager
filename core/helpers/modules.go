package helpers

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

const FieldModules = "modules"

// moduleMarker maps a manifest/lock file at the project root to an ecosystem
// label. Only the project root is inspected; nested manifests are ignored so
// monorepos with vendored deps don't light up every ecosystem.
type moduleMarker struct {
	file  string
	label string
}

var moduleMarkers = []moduleMarker{
	{"go.mod", "Go"},
	{"package.json", "npm"},
	{"composer.json", "PHP"},
	{"Cargo.toml", "Rust"},
	{"pyproject.toml", "Python"},
	{"setup.py", "Python"},
	{"requirements.txt", "Python"},
	{"Pipfile", "Python"},
	{"Gemfile", "Ruby"},
	{"pom.xml", "Maven"},
	{"build.gradle", "Gradle"},
	{"build.gradle.kts", "Gradle"},
	{"mix.exs", "Elixir"},
	{"pubspec.yaml", "Dart"},
	{"Package.swift", "Swift"},
	{"CMakeLists.txt", "CMake"},
	{"Makefile", "Make"},
	{"Dockerfile", "Docker"},
	{"docker-compose.yml", "Compose"},
	{"docker-compose.yaml", "Compose"},
	{"compose.yml", "Compose"},
	{"compose.yaml", "Compose"},
	{"project.godot", "Godot"},
	{"deno.json", "Deno"},
	{"deno.jsonc", "Deno"},
	{"bun.lockb", "Bun"},
	{"flake.nix", "Nix"},
	{"Chart.yaml", "Helm"},
	{"terraform.tf", "Terraform"},
	{"main.tf", "Terraform"},
}

type ModulesHelper struct{}

func (ModulesHelper) Name() string { return "modules" }

func (ModulesHelper) Fields() []Field {
	return []Field{{
		ID:         FieldModules,
		Label:      "Modules",
		Kind:       FieldMulti,
		Sortable:   false,
		Filterable: true,
	}}
}

func (ModulesHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	labels := detectModules(projectPath)
	return map[string]any{
		FieldModules: labels,
	}, nil
}

// detectModules returns the deduplicated, sorted ecosystem labels for the
// manifest files present at projectPath.
func detectModules(projectPath string) []string {
	seen := map[string]struct{}{}
	for _, mk := range moduleMarkers {
		if _, ok := seen[mk.label]; ok {
			continue
		}
		info, err := os.Stat(filepath.Join(projectPath, mk.file))
		if err != nil || info.IsDir() {
			continue
		}
		seen[mk.label] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for label := range seen {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}
