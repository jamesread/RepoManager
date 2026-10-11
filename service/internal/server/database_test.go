package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/config"
	appstore "github.com/jamesread/RepoManager/service/internal/store"
	"github.com/stretchr/testify/require"
)

func TestGetDatabaseStatus(t *testing.T) {
	ctx := context.Background()
	dsn := "file:" + t.TempDir() + "/test.db?_foreign_keys=on"
	db, err := appstore.Open(appstore.DriverSQLite, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.DB.Close() })

	_, err = db.DB.ExecContext(ctx, `CREATE TABLE migrations (id TEXT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `INSERT INTO migrations (id) VALUES (?)`, config.RequiredMigration)
	require.NoError(t, err)

	s := New(&config.Config{SiteTitle: "Test"}, nil, nil, nil, nil, db)
	res, err := s.GetDatabaseStatus(ctx, connect.NewRequest(&repomanagerv1.DatabaseStatusRequest{}))
	require.NoError(t, err)
	require.True(t, res.Msg.GetConnected())
	require.Equal(t, appstore.DriverSQLite, res.Msg.GetDriver())
	require.Equal(t, config.RequiredMigration, res.Msg.GetRequiredMigration())
	require.True(t, res.Msg.GetMigrationsOk())
}
