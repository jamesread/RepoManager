package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveDatabaseURL_relativeFromConfigDir(t *testing.T) {
	cfg := &Config{
		ConfigDir:   "/project/var/config",
		DatabaseURL: "file:tmp/repomanager.db?_foreign_keys=on",
	}
	got := ResolveDatabaseURL(cfg)
	require.True(t, strings.HasPrefix(got, "file:"))
	require.Contains(t, got, filepath.Join("project", "tmp", "repomanager.db"))
	require.Contains(t, got, "_foreign_keys=on")
}

func TestResolveDatabaseURL_absoluteUnchanged(t *testing.T) {
	cfg := &Config{
		DatabaseURL: "file:/var/lib/repomanager.db",
	}
	require.Equal(t, "file:/var/lib/repomanager.db", ResolveDatabaseURL(cfg))
}
