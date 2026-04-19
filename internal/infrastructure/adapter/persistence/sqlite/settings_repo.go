package sqlite

import (
	"context"
	"database/sql"
)

type SQLiteAppSettingsRepository struct {
	db *sql.DB
}

func NewSQLiteAppSettingsRepository(db *sql.DB) *SQLiteAppSettingsRepository {
	return &SQLiteAppSettingsRepository{db: db}
}

func (r *SQLiteAppSettingsRepository) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (r *SQLiteAppSettingsRepository) SetSetting(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO app_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
