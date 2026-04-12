package sqlite

import (
	"context"
	"database/sql"
	"subsync/internal/core/domain/entity"
)

type SQLiteWatchDirRepository struct {
	db *sql.DB
}

func NewSQLiteWatchDirRepository(db *sql.DB) *SQLiteWatchDirRepository {
	return &SQLiteWatchDirRepository{db: db}
}

func (r *SQLiteWatchDirRepository) FindAll(ctx context.Context) ([]*entity.WatchDir, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, path, enabled, created_at FROM watch_dirs ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []*entity.WatchDir
	for rows.Next() {
		var id int
		var path string
		var enabled bool
		var createdAtStr sql.NullString
		if err := rows.Scan(&id, &path, &enabled, &createdAtStr); err != nil {
			return nil, err
		}
		ca := parseTime(createdAtStr)
		dirs = append(dirs, entity.RestoreWatchDir(id, path, enabled, ca))
	}
	return dirs, rows.Err()
}

func (r *SQLiteWatchDirRepository) FindEnabled(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT path FROM watch_dirs WHERE enabled = 1 ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, rows.Err()
}

func (r *SQLiteWatchDirRepository) Save(ctx context.Context, w *entity.WatchDir) error {
	if w.ID() == 0 {
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO watch_dirs (path, enabled, created_at) VALUES (?, ?, ?)`,
			w.Path(), w.IsEnabled(), w.CreatedAt(),
		)
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE watch_dirs SET path = ?, enabled = ? WHERE id = ?`,
		w.Path(), w.IsEnabled(), w.ID(),
	)
	return err
}

func (r *SQLiteWatchDirRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM watch_dirs WHERE id = ?`, id)
	return err
}

func (r *SQLiteWatchDirRepository) FindByID(ctx context.Context, id int) (*entity.WatchDir, error) {
	var wdID int
	var path string
	var enabled bool
	var createdAtStr sql.NullString
	err := r.db.QueryRowContext(ctx, `SELECT id, path, enabled, created_at FROM watch_dirs WHERE id = ?`, id).
		Scan(&wdID, &path, &enabled, &createdAtStr)
	if err != nil {
		return nil, err
	}
	return entity.RestoreWatchDir(wdID, path, enabled, parseTime(createdAtStr)), nil
}
