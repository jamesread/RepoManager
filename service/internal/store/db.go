package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	mysqldriver "github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

const (
	DriverSQLite = "sqlite3"
	DriverMySQL  = "mysql"
)

type Database struct {
	Driver string
	DSN    string
	DB     *sql.DB
}

func DetectDriver(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if strings.HasPrefix(dsn, "file:") {
		return DriverSQLite
	}
	return DriverMySQL
}

func Open(driver, dsn string) (*Database, error) {
	driver = strings.TrimSpace(driver)
	if driver == "" {
		driver = DetectDriver(dsn)
	}
	if driver == DriverSQLite {
		db, err := openSQLite(dsn)
		if err != nil {
			return nil, err
		}
		return &Database{Driver: DriverSQLite, DSN: dsn, DB: db}, nil
	}
	if driver == DriverMySQL {
		db, err := openMySQL(dsn)
		if err != nil {
			return nil, err
		}
		return &Database{Driver: DriverMySQL, DSN: dsn, DB: db}, nil
	}
	return nil, fmt.Errorf("unsupported database driver %q", driver)
}

func openSQLite(dsn string) (*sql.DB, error) {
	if stringsHasPrefixFile(dsn) {
		if err := os.MkdirAll(filepath.Dir(stringsTrimFilePrefix(dsn)), 0o755); err != nil {
			return nil, err
		}
	}
	db, err := sql.Open(DriverSQLite, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func openMySQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open(DriverMySQL, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// OpenSQLite is kept for tests and legacy call sites.
func OpenSQLite(dsn string) (*sql.DB, error) {
	return openSQLite(dsn)
}

func AssertMigration(ctx context.Context, db *sql.DB, required string) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE id = ?`, required).Scan(&count)
	if err != nil {
		return fmt.Errorf("database migration check failed (run make migrate): %w", err)
	}
	if count == 0 {
		return fmt.Errorf("required migration %q is not applied; run: make migrate", required)
	}
	return nil
}

func HasMigration(ctx context.Context, db *sql.DB, id string) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM migrations WHERE id = ?`, id).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func LatestMigration(ctx context.Context, db *sql.DB) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT id FROM migrations ORDER BY id DESC LIMIT 1`).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

func ListMigrations(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT id FROM migrations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

type Status struct {
	Driver            string
	DSNRedacted       string
	Host              string
	Database          string
	Connected         bool
	ConnectionError   string
	RequiredMigration string
	LatestMigration   string
	MigrationsOK      bool
	AppliedMigrations []string
}

func BuildStatus(ctx context.Context, db *Database, requiredMigration string) Status {
	st := Status{
		Driver:            db.Driver,
		DSNRedacted:       RedactDSN(db.Driver, db.DSN),
		RequiredMigration: requiredMigration,
	}
	if db.Driver == DriverMySQL {
		st.Host, st.Database = parseMySQLHostDatabase(db.DSN)
	}
	if err := db.DB.PingContext(ctx); err != nil {
		st.ConnectionError = err.Error()
		return st
	}
	st.Connected = true
	latest, err := LatestMigration(ctx, db.DB)
	if err == nil {
		st.LatestMigration = latest
	}
	ok, err := HasMigration(ctx, db.DB, requiredMigration)
	if err == nil {
		st.MigrationsOK = ok
	}
	applied, err := ListMigrations(ctx, db.DB)
	if err == nil {
		st.AppliedMigrations = applied
	}
	return st
}

func RedactDSN(driver, dsn string) string {
	switch driver {
	case DriverSQLite:
		return dsn
	default:
		return redactMySQLDSN(dsn)
	}
}

var mysqlPasswordRE = regexp.MustCompile(`^([^:@/]+):([^@]*)@`)

func redactMySQLDSN(dsn string) string {
	if mysqlPasswordRE.MatchString(dsn) {
		return mysqlPasswordRE.ReplaceAllString(dsn, `$1:***@`)
	}
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		return dsn
	}
	if _, hasPass := u.User.Password(); hasPass {
		u.User = url.UserPassword(u.User.Username(), "***")
	}
	return u.String()
}

func parseMySQLHostDatabase(dsn string) (host, database string) {
	cfg, err := mysqldriver.ParseDSN(dsn)
	if err != nil {
		return "", ""
	}
	host = cfg.Addr
	if host == "" {
		host = "localhost"
	}
	return host, cfg.DBName
}

func stringsHasPrefixFile(dsn string) bool {
	return len(dsn) >= 5 && dsn[:5] == "file:"
}

func stringsTrimFilePrefix(dsn string) string {
	path := dsn[5:]
	if i := indexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return path
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
