package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jamesread/RepoManager/service/internal/store"
)

// ResolveDatabaseURL turns file: DSN paths into absolute paths anchored at the
// repository root (parent of var/ when -configdir var/config, else nearest go.mod).
func ResolveDatabaseDriver(cfg *Config, dsn string) string {
	if driver := strings.TrimSpace(cfg.DatabaseDriver); driver != "" {
		return driver
	}
	return store.DetectDriver(dsn)
}

func ResolveDatabaseURL(cfg *Config) string {
	dsn := strings.TrimSpace(cfg.DatabaseURL)
	if !strings.HasPrefix(dsn, "file:") {
		return dsn
	}
	rest := dsn[len("file:"):]
	path := rest
	query := ""
	if i := strings.Index(rest, "?"); i >= 0 {
		path = rest[:i]
		query = rest[i:]
	}
	if filepath.IsAbs(path) {
		return dsn
	}
	abs, err := filepath.Abs(filepath.Join(repoRoot(cfg.ConfigDir), path))
	if err != nil {
		return dsn
	}
	return "file:" + filepath.ToSlash(abs) + query
}

func repoRoot(configDir string) string {
	if configDir != "" {
		return filepath.Clean(filepath.Join(configDir, "..", ".."))
	}
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd
		}
		dir = parent
	}
}
