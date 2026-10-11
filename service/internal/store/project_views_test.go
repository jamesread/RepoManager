package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func openTestDBWithProjectViews(t *testing.T) *Database {
	t.Helper()
	ctx := context.Background()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db") + "?_foreign_keys=on"
	db, err := Open(DriverSQLite, dsn)
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
	_, err = db.DB.ExecContext(ctx, `INSERT INTO user_accounts (username) VALUES ('alice')`)
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

func TestProjectViewStoreCreateListDefault(t *testing.T) {
	ctx := context.Background()
	db := openTestDBWithProjectViews(t)
	views := NewProjectViewStore(db.DB, db.Driver)

	created, defaultID, err := views.Create(ctx, 1, "Managers", `{"version":1}`, true)
	require.NoError(t, err)
	require.Equal(t, created.ID, defaultID)
	require.True(t, created.IsDefault)

	list, nextDefault, err := views.ListByUser(ctx, 1)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, created.ID, nextDefault)

	_, _, err = views.Create(ctx, 1, "Managers", `{}`, false)
	require.ErrorIs(t, err, ErrProjectViewNameUsed)
}

func TestProjectViewStoreSetDefaultAndDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDBWithProjectViews(t)
	views := NewProjectViewStore(db.DB, db.Driver)

	first, _, err := views.Create(ctx, 1, "One", `{}`, false)
	require.NoError(t, err)
	second, _, err := views.Create(ctx, 1, "Two", `{}`, true)
	require.NoError(t, err)

	defaultID, err := views.SetDefault(ctx, 1, first.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, defaultID)

	got, err := views.GetByID(ctx, 1, second.ID)
	require.NoError(t, err)
	require.False(t, got.IsDefault)

	defaultID, err = views.Delete(ctx, 1, first.ID)
	require.NoError(t, err)
	require.Equal(t, 0, defaultID)
}
