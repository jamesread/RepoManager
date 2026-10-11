package iamstore

import (
	"database/sql"

	armstore "github.com/jamesread/armature-iam/store"
	iammysql "github.com/jamesread/RepoManager/service/internal/iamstore/mysql"
	iamsqlite "github.com/jamesread/armature-iam/store/sqlite"
)

// New returns an IAM store for the given SQL driver name.
func New(driver string, db *sql.DB) armstore.Store {
	if driver == "mysql" {
		return iammysql.New(db)
	}
	return iamsqlite.New(db)
}
