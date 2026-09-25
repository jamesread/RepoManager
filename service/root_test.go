package main

import (
	"path/filepath"
	"testing"

	"github.com/jamesread/RepoManager/core/project"
	"github.com/jamesread/RepoManager/core/store"
	"github.com/stretchr/testify/require"
)

func TestResolveServiceRootUsesLastExisting(t *testing.T) {
	st := newServiceStore(t)
	older := t.TempDir()
	newer := t.TempDir()
	require.NoError(t, st.RememberRoot(older))
	require.NoError(t, st.RememberRoot(newer))

	got, err := resolveServiceRoot(st, "")
	require.NoError(t, err)
	newerNorm, err := project.NormalizeRoot(newer)
	require.NoError(t, err)
	require.Equal(t, newerNorm, got)
	require.Equal(t, newerNorm, st.RecentRoots()[0])
}

func TestResolveServiceRootSkipsMissing(t *testing.T) {
	st := newServiceStore(t)
	alive := t.TempDir()
	missing := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, st.RememberRoot(alive))
	require.NoError(t, st.RememberRoot(missing))

	got, err := resolveServiceRoot(st, "")
	require.NoError(t, err)
	aliveNorm, err := project.NormalizeRoot(alive)
	require.NoError(t, err)
	require.Equal(t, aliveNorm, got)
	require.Equal(t, aliveNorm, st.RecentRoots()[0])
}

func TestResolveServiceRootFlagOverrides(t *testing.T) {
	st := newServiceStore(t)
	saved := t.TempDir()
	flag := t.TempDir()
	require.NoError(t, st.RememberRoot(saved))

	got, err := resolveServiceRoot(st, flag)
	require.NoError(t, err)
	flagNorm, err := project.NormalizeRoot(flag)
	require.NoError(t, err)
	savedNorm, err := project.NormalizeRoot(saved)
	require.NoError(t, err)
	require.Equal(t, flagNorm, got)
	require.Equal(t, []string{flagNorm, savedNorm}, st.RecentRoots())
}

func newServiceStore(t *testing.T) *store.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	st, err := store.Load()
	require.NoError(t, err)
	return st
}
