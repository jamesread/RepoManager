package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jamesread/RepoManager/service/internal/cvar"
)

type CvarRow struct {
	Key         string
	MainType    string
	ValueInt    sql.NullInt64
	ValueString sql.NullString
	Title       string
	Description string
	Category    string
	Ordinal     int
}

type CvarStore struct {
	db     *sql.DB
	driver string
}

func NewCvarStore(db *sql.DB, driver string) *CvarStore {
	if driver == "" {
		driver = DriverSQLite
	}
	return &CvarStore{db: db, driver: driver}
}

func (s *CvarStore) EnsureDefaults(ctx context.Context, siteTitle string) error {
	upsert := cvarUpsertSQL(s.driver)
	for _, d := range cvar.Defaults(siteTitle) {
		_, err := s.db.ExecContext(ctx, upsert,
			d.Key, d.ValueInt, d.ValueString, d.MainType,
			d.Title, d.Description, d.Category, d.Ordinal,
		)
		if err != nil {
			return fmt.Errorf("ensure cvar %s: %w", d.Key, err)
		}
	}
	return nil
}

func (s *CvarStore) List(ctx context.Context) ([]CvarRow, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT cvar_key, cvar_main_type, cvar_value_int, cvar_value_string,
       cvar_title, cvar_description, cvar_category, cvar_ordinal
FROM cvars ORDER BY cvar_ordinal, cvar_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CvarRow
	for rows.Next() {
		var row CvarRow
		if err := rows.Scan(
			&row.Key, &row.MainType, &row.ValueInt, &row.ValueString,
			&row.Title, &row.Description, &row.Category, &row.Ordinal,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *CvarStore) Update(ctx context.Context, key string, valueInt int, valueString string) error {
	res, err := s.db.ExecContext(ctx, `
UPDATE cvars SET cvar_value_int = ?, cvar_value_string = NULLIF(?, '')
WHERE cvar_key = ?`, valueInt, valueString, key)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *CvarStore) GetString(ctx context.Context, key, fallback string) string {
	var v sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT cvar_value_string FROM cvars WHERE cvar_key = ?`, key).Scan(&v)
	if err != nil || !v.Valid || v.String == "" {
		return fallback
	}
	return v.String
}

func (s *CvarStore) GetBool(ctx context.Context, key string, fallback bool) bool {
	var v sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT cvar_value_int FROM cvars WHERE cvar_key = ?`, key).Scan(&v)
	if err != nil || !v.Valid {
		return fallback
	}
	return v.Int64 != 0
}

func (s *CvarStore) LatestMigration(ctx context.Context) (string, error) {
	return LatestMigration(ctx, s.db)
}

func cvarUpsertSQL(driver string) string {
	if driver == DriverMySQL {
		return `
INSERT INTO cvars (
  cvar_key, cvar_value_int, cvar_value_string, cvar_main_type,
  cvar_title, cvar_description, cvar_category, cvar_ordinal
) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
ON DUPLICATE KEY UPDATE
  cvar_title = VALUES(cvar_title),
  cvar_description = VALUES(cvar_description),
  cvar_category = VALUES(cvar_category),
  cvar_ordinal = VALUES(cvar_ordinal)`
	}
	return `
INSERT INTO cvars (
  cvar_key, cvar_value_int, cvar_value_string, cvar_main_type,
  cvar_title, cvar_description, cvar_category, cvar_ordinal
) VALUES (?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)
ON CONFLICT(cvar_key) DO UPDATE SET
  cvar_title = excluded.cvar_title,
  cvar_description = excluded.cvar_description,
  cvar_category = excluded.cvar_category,
  cvar_ordinal = excluded.cvar_ordinal`
}
