package sqlite

import (
	"database/sql"
	_ "embed"
)

//go:embed migrations/001_init.sql
var initSQL string

//go:embed migrations/002_add_uuid.sql
var migration002SQL string

//go:embed migrations/003_add_watch_dirs.sql
var migration003SQL string

//go:embed migrations/004_apikey_limits.sql
var migration004SQL string

//go:embed migrations/005_update_model_names.sql
var migration005SQL string

func Migrate(db *sql.DB) error {
	if _, err := db.Exec(initSQL); err != nil {
		return err
	}
	_, _ = db.Exec(migration002SQL)
	_, _ = db.Exec(migration003SQL)
	_, _ = db.Exec(migration004SQL)
	_, _ = db.Exec(migration005SQL)
	return nil
}
