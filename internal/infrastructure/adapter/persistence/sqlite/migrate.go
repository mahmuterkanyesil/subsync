package sqlite

import (
	"database/sql"
	_ "embed"
)

//go:embed migrations/001_init.sql
var initSQL string

//go:embed migrations/002_add_uuid.sql
var migration002SQL string

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(initSQL); err != nil {
		return err
	}
	// 002: ALTER TABLE — mevcut sütun varsa hatayı yoksay (idempotent)
	_, _ = db.Exec(migration002SQL)
	return nil
}
