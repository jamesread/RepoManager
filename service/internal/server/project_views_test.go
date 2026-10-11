package server

import (
	"context"
	"path/filepath"
	"testing"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	armlayer "github.com/jamesread/armature-iam/layer"
	"github.com/jamesread/armature-iam/store"
	repomanagerv1 "github.com/jamesread/RepoManager/service/gen/repomanager/v1"
	"github.com/jamesread/RepoManager/service/internal/config"
	appstore "github.com/jamesread/RepoManager/service/internal/store"
	"github.com/stretchr/testify/require"
)

func TestProjectViewsCRUD(t *testing.T) {
	ctx := authn.SetInfo(context.Background(), &armlayer.AuthenticatedUser{
		User: &store.UserAccountRow{ID: 1, Username: "alice"},
	})
	db := openProjectViewsServerDB(t)
	views := appstore.NewProjectViewStore(db.DB, db.Driver)
	s := New(&config.Config{SiteTitle: "Test"}, nil, nil, nil, views, db)

	saveRes, err := s.SaveProjectView(ctx, connect.NewRequest(&repomanagerv1.SaveProjectViewRequest{
		Name:          "Managers",
		StateJson:     `{"version":1,"icons":{"sortBy":"name"}}`,
		SetAsDefault:  true,
	}))
	require.NoError(t, err)
	require.Equal(t, int32(1), saveRes.Msg.GetView().GetId())
	require.True(t, saveRes.Msg.GetView().GetIsDefault())

	listRes, err := s.ListProjectViews(ctx, connect.NewRequest(&repomanagerv1.ListProjectViewsRequest{}))
	require.NoError(t, err)
	require.Len(t, listRes.Msg.GetViews(), 1)
	require.Equal(t, int32(1), listRes.Msg.GetDefaultViewId())

	_, err = s.SetDefaultProjectView(ctx, connect.NewRequest(&repomanagerv1.SetDefaultProjectViewRequest{}))
	require.NoError(t, err)

	delRes, err := s.DeleteProjectView(ctx, connect.NewRequest(&repomanagerv1.DeleteProjectViewRequest{ViewId: 1}))
	require.NoError(t, err)
	require.Equal(t, int32(0), delRes.Msg.GetDefaultViewId())
}

func openProjectViewsServerDB(t *testing.T) *appstore.Database {
	t.Helper()
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_foreign_keys=on"
	db, err := appstore.Open(appstore.DriverSQLite, dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.DB.Close() })

	_, err = db.DB.ExecContext(ctx, `
CREATE TABLE user_accounts (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  username TEXT NOT NULL,
  password_hash TEXT NOT NULL DEFAULT '',
  created_by TEXT NOT NULL DEFAULT 'admin-created'
)`)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `INSERT INTO user_accounts (id, username) VALUES (1, 'alice')`)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `
CREATE TABLE project_views (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  user_account_id INTEGER NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  state_json TEXT NOT NULL,
  is_default INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
)`)
	require.NoError(t, err)
	_, err = db.DB.ExecContext(ctx, `CREATE UNIQUE INDEX idx_project_views_user_name ON project_views(user_account_id, name COLLATE NOCASE)`)
	require.NoError(t, err)
	return db
}
