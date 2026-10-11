package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jamesread/RepoManager/core/engine"
	"github.com/jamesread/RepoManager/core/store"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/config"
	"github.com/stretchr/testify/require"
)

func TestListProjects(t *testing.T) {
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

	s := New(&config.Config{SiteTitle: "Test"}, eng, nil, nil, nil, nil)
	res, err := s.ListProjects(context.Background(), connect.NewRequest(&repomanagerv1.ListProjectsRequest{}))
	require.NoError(t, err)
	require.Equal(t, root, res.Msg.GetRoot())
	require.NotEmpty(t, res.Msg.GetFields())
	require.NotEmpty(t, res.Msg.GetProjects())
}
