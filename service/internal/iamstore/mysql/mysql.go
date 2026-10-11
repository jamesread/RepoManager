package mysql

import (
	"context"
	"database/sql"
)

// MySQL implements store.Store on a shared *sql.DB.
type MySQL struct {
	db *sql.DB
}

// New wraps an existing MySQL connection. The caller applies migrations.
func New(db *sql.DB) *MySQL {
	return &MySQL{db: db}
}

func (s *MySQL) DB() *sql.DB {
	return s.db
}

func (s *MySQL) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}
