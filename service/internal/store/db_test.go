package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectDriver(t *testing.T) {
	require.Equal(t, DriverSQLite, DetectDriver("file:tmp/repomanager.db"))
	require.Equal(t, DriverMySQL, DetectDriver("repomanager:secret@tcp(127.0.0.1:3306)/repomanager?parseTime=true"))
}

func TestRedactMySQLDSN(t *testing.T) {
	got := RedactDSN(DriverMySQL, "repomanager:secret@tcp(127.0.0.1:3306)/repomanager?parseTime=true")
	require.Equal(t, "repomanager:***@tcp(127.0.0.1:3306)/repomanager?parseTime=true", got)
}

func TestParseMySQLHostDatabase(t *testing.T) {
	host, db := parseMySQLHostDatabase("repomanager:secret@tcp(db.example:3306)/repomanager?parseTime=true")
	require.Equal(t, "db.example:3306", host)
	require.Equal(t, "repomanager", db)
}
