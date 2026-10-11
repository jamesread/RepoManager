package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const MaxProjectViewsPerUser = 20

var (
	ErrProjectViewNotFound = errors.New("project view not found")
	ErrProjectViewNameUsed = errors.New("project view name already exists")
	ErrProjectViewLimit    = errors.New("project view limit reached")
)

type ProjectViewRow struct {
	ID        int
	UserID    int
	Name      string
	StateJSON string
	IsDefault bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectViewStore struct {
	db     *sql.DB
	driver string
}

func NewProjectViewStore(db *sql.DB, driver string) *ProjectViewStore {
	if driver == "" {
		driver = DriverSQLite
	}
	return &ProjectViewStore{db: db, driver: driver}
}

func (s *ProjectViewStore) ListByUser(ctx context.Context, userID int) ([]ProjectViewRow, int, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, user_account_id, name, state_json, is_default, created_at, updated_at
FROM project_views
WHERE user_account_id = ?
ORDER BY name COLLATE NOCASE`, userID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []ProjectViewRow
	defaultID := 0
	for rows.Next() {
		row, err := scanProjectView(rows)
		if err != nil {
			return nil, 0, err
		}
		if row.IsDefault {
			defaultID = row.ID
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, defaultID, nil
}

func (s *ProjectViewStore) GetByID(ctx context.Context, userID, id int) (ProjectViewRow, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, user_account_id, name, state_json, is_default, created_at, updated_at
FROM project_views
WHERE user_account_id = ? AND id = ?`, userID, id)
	view, err := scanProjectView(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ProjectViewRow{}, ErrProjectViewNotFound
	}
	return view, err
}

func (s *ProjectViewStore) Create(ctx context.Context, userID int, name, stateJSON string, setDefault bool) (ProjectViewRow, int, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ProjectViewRow{}, 0, fmt.Errorf("name is required")
	}
	count, err := s.countByUser(ctx, userID)
	if err != nil {
		return ProjectViewRow{}, 0, err
	}
	if count >= MaxProjectViewsPerUser {
		return ProjectViewRow{}, 0, ErrProjectViewLimit
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ProjectViewRow{}, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `
INSERT INTO project_views (user_account_id, name, state_json, is_default)
VALUES (?, ?, ?, ?)`, userID, name, stateJSON, boolInt(setDefault))
	if err != nil {
		if isUniqueViolation(err) {
			return ProjectViewRow{}, 0, ErrProjectViewNameUsed
		}
		return ProjectViewRow{}, 0, err
	}
	id64, err := res.LastInsertId()
	if err != nil {
		return ProjectViewRow{}, 0, err
	}
	id := int(id64)

	defaultID := 0
	if setDefault {
		if err := clearDefaultViews(ctx, tx, userID); err != nil {
			return ProjectViewRow{}, 0, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE project_views SET is_default = 1 WHERE id = ? AND user_account_id = ?`, id, userID); err != nil {
			return ProjectViewRow{}, 0, err
		}
		defaultID = id
	} else {
		defaultID, err = s.defaultIDTx(ctx, tx, userID)
		if err != nil {
			return ProjectViewRow{}, 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return ProjectViewRow{}, 0, err
	}
	view, err := s.GetByID(ctx, userID, id)
	return view, defaultID, err
}

func (s *ProjectViewStore) Delete(ctx context.Context, userID, id int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.ExecContext(ctx, `DELETE FROM project_views WHERE user_account_id = ? AND id = ?`, userID, id)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, ErrProjectViewNotFound
	}

	defaultID, err := s.defaultIDTx(ctx, tx, userID)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return defaultID, nil
}

func (s *ProjectViewStore) SetDefault(ctx context.Context, userID, id int) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := clearDefaultViews(ctx, tx, userID); err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, tx.Commit()
	}

	res, err := tx.ExecContext(ctx, `UPDATE project_views SET is_default = 1 WHERE user_account_id = ? AND id = ?`, userID, id)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if affected == 0 {
		return 0, ErrProjectViewNotFound
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *ProjectViewStore) countByUser(ctx context.Context, userID int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM project_views WHERE user_account_id = ?`, userID).Scan(&count)
	return count, err
}

func (s *ProjectViewStore) defaultIDTx(ctx context.Context, tx *sql.Tx, userID int) (int, error) {
	var id sql.NullInt64
	err := tx.QueryRowContext(ctx, `
SELECT id FROM project_views WHERE user_account_id = ? AND is_default = 1 LIMIT 1`, userID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if id.Valid {
		return int(id.Int64), nil
	}
	return 0, nil
}

func clearDefaultViews(ctx context.Context, tx *sql.Tx, userID int) error {
	_, err := tx.ExecContext(ctx, `UPDATE project_views SET is_default = 0 WHERE user_account_id = ?`, userID)
	return err
}

func scanProjectView(row interface {
	Scan(dest ...any) error
}) (ProjectViewRow, error) {
	var view ProjectViewRow
	var isDefault int
	var createdAt, updatedAt string
	if err := row.Scan(
		&view.ID,
		&view.UserID,
		&view.Name,
		&view.StateJSON,
		&isDefault,
		&createdAt,
		&updatedAt,
	); err != nil {
		return ProjectViewRow{}, err
	}
	view.IsDefault = isDefault != 0
	view.CreatedAt = parseDBTime(createdAt)
	view.UpdatedAt = parseDBTime(updatedAt)
	return view, nil
}

func parseDBTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t
		}
	}
	return time.Time{}
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func isUniqueViolation(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}
