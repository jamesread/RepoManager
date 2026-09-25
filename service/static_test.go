package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStaticDirCandidatesPreferBinaryDir(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	binDir := filepath.Dir(exe)
	if resolved, err := filepath.EvalSymlinks(binDir); err == nil {
		binDir = resolved
	}

	cands := staticDirCandidates()
	require.GreaterOrEqual(t, len(cands), 3)
	require.Equal(t, filepath.Join(binDir, "frontend", "dist"), cands[0])
	require.Equal(t, filepath.Join(binDir, "..", "frontend", "dist"), cands[1])
}

func TestFindStaticDirBesideBinary(t *testing.T) {
	t.Setenv("PLIST_STATIC_DIR", "")

	exe, err := os.Executable()
	require.NoError(t, err)
	binDir := filepath.Dir(exe)
	if resolved, err := filepath.EvalSymlinks(binDir); err == nil {
		binDir = resolved
	}

	dist := filepath.Join(binDir, "frontend", "dist")
	require.NoError(t, os.MkdirAll(dist, 0o755))
	t.Cleanup(func() {
		_ = os.RemoveAll(filepath.Join(binDir, "frontend"))
	})

	got := findStaticDir()
	require.Equal(t, dist, got)
}
