-- +migrate Up

CREATE TABLE project_views (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  user_account_id INTEGER NOT NULL REFERENCES user_accounts(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  state_json TEXT NOT NULL,
  is_default INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_project_views_user_name ON project_views(user_account_id, name COLLATE NOCASE);
CREATE INDEX idx_project_views_user_default ON project_views(user_account_id, is_default);

-- +migrate Down

DROP TABLE IF EXISTS project_views;
