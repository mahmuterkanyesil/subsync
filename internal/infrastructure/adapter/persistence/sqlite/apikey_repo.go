package sqlite

import (
	"context"
	"database/sql"
	"subsync/internal/core/domain/entity"
	"time"
)

type SQLiteAPIKeyRepository struct {
	db *sql.DB
}

func NewSQLiteAPIKeyRepository(db *sql.DB) *SQLiteAPIKeyRepository {
	return &SQLiteAPIKeyRepository{db: db}
}

func (r *SQLiteAPIKeyRepository) Save(ctx context.Context, k *entity.APIKey) error {
	now := time.Now()
	if k.ID() == 0 {
		query := `INSERT INTO api_keys (service, key_value, is_active, is_quota_exceeded, quota_reset_time, request_made, last_used_at, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := r.db.ExecContext(ctx, query,
			k.Service(), k.KeyValue(), k.IsActive(), k.IsQuotaExceeded(),
			k.QuotaResetTime(), k.RequestMade(), k.LastUsedAt(), k.LastError(), now, now,
		)
		return err
	}
	query := `UPDATE api_keys SET is_active=?, is_quota_exceeded=?, quota_reset_time=?, request_made=?, last_used_at=?, last_error=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query,
		k.IsActive(), k.IsQuotaExceeded(), k.QuotaResetTime(),
		k.RequestMade(), k.LastUsedAt(), k.LastError(), now, k.ID(),
	)
	return err
}

func (r *SQLiteAPIKeyRepository) FindByID(ctx context.Context, id int) (*entity.APIKey, error) {
	query := `SELECT id, service, key_value, is_active, is_quota_exceeded, quota_reset_time, request_made, last_used_at, last_error, created_at, updated_at FROM api_keys WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanAPIKey(row)
}

func (r *SQLiteAPIKeyRepository) FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error) {
	_ = r.ResetExpiredQuotas(ctx)
	query := `
		SELECT id, service, key_value, is_active, is_quota_exceeded, quota_reset_time, request_made, last_used_at, last_error, created_at, updated_at
		FROM api_keys
		WHERE service = ? AND is_active = 1 AND is_quota_exceeded = 0
		ORDER BY last_used_at ASC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, service)
	return scanAPIKey(row)
}

func (r *SQLiteAPIKeyRepository) ResetExpiredQuotas(ctx context.Context) error {
	query := `
		UPDATE api_keys
		SET is_quota_exceeded = 0, quota_reset_time = NULL, updated_at = ?
		WHERE is_quota_exceeded = 1 AND quota_reset_time <= ?
	`
	now := time.Now()
	_, err := r.db.ExecContext(ctx, query, now, now)
	return err
}

func scanAPIKey(row *sql.Row) (*entity.APIKey, error) {
	var id, requestMade int
	var service, keyValue, lastError string
	var isActive, isQuotaExceeded bool
	var quotaResetTime *time.Time
	var lastUsedAt *time.Time
	var createdAt, updatedAt time.Time

	err := row.Scan(&id, &service, &keyValue, &isActive, &isQuotaExceeded, &quotaResetTime, &requestMade, &lastUsedAt, &lastError, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	return entity.RestoreAPIKey(id, service, keyValue, isActive, isQuotaExceeded, quotaResetTime, requestMade, lastUsedAt, lastError, createdAt, updatedAt)
}
