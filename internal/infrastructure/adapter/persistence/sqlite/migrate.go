package sqlite

import (
	"database/sql"
	_ "embed"
)

//go:embed migrations/001_init.sql
var initSQL string

func Migrate(db *sql.DB) error {
	_, err := db.Exec(initSQL)
	return err
}
