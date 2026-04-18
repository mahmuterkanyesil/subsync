package sqlite

import (
	"context"
	"database/sql"
	"subsync/internal/core/application/port"
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
		query := `INSERT INTO api_keys (service, key_value, model, is_active, is_quota_exceeded, quota_reset_time, rpm_limit, tpm_limit, rpd_limit, request_made, last_used_at, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		_, err := r.db.ExecContext(ctx, query,
			k.Service(), k.KeyValue(), k.Model(), k.IsActive(), k.IsQuotaExceeded(),
			k.QuotaResetTime(), k.RPMLimit(), k.TPMLimit(), k.RPDLimit(),
			k.RequestMade(), k.LastUsedAt(), k.LastError(), now, now,
		)
		return err
	}
	query := `UPDATE api_keys SET model=?, is_active=?, is_quota_exceeded=?, quota_reset_time=?, rpm_limit=?, tpm_limit=?, rpd_limit=?, request_made=?, last_used_at=?, last_error=?, updated_at=? WHERE id=?`
	_, err := r.db.ExecContext(ctx, query,
		k.Model(), k.IsActive(), k.IsQuotaExceeded(), k.QuotaResetTime(),
		k.RPMLimit(), k.TPMLimit(), k.RPDLimit(),
		k.RequestMade(), k.LastUsedAt(), k.LastError(), now, k.ID(),
	)
	return err
}

func (r *SQLiteAPIKeyRepository) FindByID(ctx context.Context, id int) (*entity.APIKey, error) {
	query := `SELECT id, service, key_value, model, is_active, is_quota_exceeded, quota_reset_time, rpm_limit, tpm_limit, rpd_limit, request_made, last_used_at, last_error, created_at, updated_at FROM api_keys WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanAPIKey(row)
}

func (r *SQLiteAPIKeyRepository) FindNextAvailable(ctx context.Context, service string) (*entity.APIKey, error) {
	_ = r.ResetExpiredQuotas(ctx)
	query := `
		SELECT id, service, key_value, model, is_active, is_quota_exceeded, quota_reset_time, rpm_limit, tpm_limit, rpd_limit, request_made, last_used_at, last_error, created_at, updated_at
		FROM api_keys
		WHERE service = ? AND is_active = 1 AND is_quota_exceeded = 0
		ORDER BY last_used_at ASC
		LIMIT 1
	`
	row := r.db.QueryRowContext(ctx, query, service)
	return scanAPIKey(row)
}

func (r *SQLiteAPIKeyRepository) ResetExpiredQuotas(ctx context.Context) error {
	now := time.Now()
	_ = r.ResetExpiredModelUsages(ctx)
	query := `
		UPDATE api_keys
		SET is_quota_exceeded = 0, quota_reset_time = NULL, request_made = 0, updated_at = ?
		WHERE is_quota_exceeded = 1 AND quota_reset_time <= ?
	`
	_, err := r.db.ExecContext(ctx, query, now, now)
	return err
}

func (r *SQLiteAPIKeyRepository) IncrementModelUsage(ctx context.Context, keyID int, model string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO api_key_model_usage (api_key_id, model, request_made, updated_at)
		 VALUES (?, ?, 1, ?)
		 ON CONFLICT(api_key_id, model) DO UPDATE
		 SET request_made = request_made + 1, updated_at = excluded.updated_at`,
		keyID, model, time.Now(),
	)
	return err
}

func (r *SQLiteAPIKeyRepository) FindAllModelUsage(ctx context.Context, keyID int) ([]port.ModelUsage, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT model, request_made FROM api_key_model_usage WHERE api_key_id = ? ORDER BY model`,
		keyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []port.ModelUsage
	for rows.Next() {
		var u port.ModelUsage
		if err := rows.Scan(&u.Model, &u.RequestMade); err != nil {
			return nil, err
		}
		result = append(result, u)
	}
	return result, rows.Err()
}

func (r *SQLiteAPIKeyRepository) ResetExpiredModelUsages(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM api_key_model_usage
		 WHERE api_key_id IN (
		     SELECT id FROM api_keys
		     WHERE is_quota_exceeded = 1 AND quota_reset_time <= ?
		 )`,
		time.Now(),
	)
	return err
}

func (r *SQLiteAPIKeyRepository) FindAll(ctx context.Context) ([]*entity.APIKey, error) {
	query := `SELECT id, service, key_value, model, is_active, is_quota_exceeded, quota_reset_time, rpm_limit, tpm_limit, rpd_limit, request_made, last_used_at, last_error, created_at, updated_at FROM api_keys ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []*entity.APIKey
	for rows.Next() {
		k, err := scanAPIKeyFromRows(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (r *SQLiteAPIKeyRepository) FindEarliestQuotaReset(ctx context.Context, service string) (*time.Time, error) {
	query := `
		SELECT MIN(quota_reset_time) FROM api_keys
		WHERE service = ? AND is_active = 1 AND is_quota_exceeded = 1 AND quota_reset_time IS NOT NULL
	`
	var s sql.NullString
	if err := r.db.QueryRowContext(ctx, query, service).Scan(&s); err != nil {
		return nil, err
	}
	if !s.Valid {
		return nil, nil
	}
	t := parseTime(s)
	return &t, nil
}

func (r *SQLiteAPIKeyRepository) Delete(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, id)
	return err
}

func scanAPIKeyFromRows(rows *sql.Rows) (*entity.APIKey, error) {
	var id, rpmLimit, tpmLimit, rpdLimit, requestMade int
	var service, keyValue, model, lastError string
	var isActive, isQuotaExceeded bool
	var quotaResetTimeStr sql.NullString
	var lastUsedAtStr sql.NullString
	var createdAtStr, updatedAtStr sql.NullString

	err := rows.Scan(&id, &service, &keyValue, &model, &isActive, &isQuotaExceeded, &quotaResetTimeStr, &rpmLimit, &tpmLimit, &rpdLimit, &requestMade, &lastUsedAtStr, &lastError, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}

	qrt := parseTimePtr(quotaResetTimeStr)
	lsu := parseTimePtr(lastUsedAtStr)
	ca := parseTime(createdAtStr)
	ua := parseTime(updatedAtStr)
	return entity.RestoreAPIKey(id, service, keyValue, model, isActive, isQuotaExceeded, qrt, rpmLimit, tpmLimit, rpdLimit, requestMade, lsu, lastError, ca, ua)
}

func scanAPIKey(row *sql.Row) (*entity.APIKey, error) {
	var id, rpmLimit, tpmLimit, rpdLimit, requestMade int
	var service, keyValue, model, lastError string
	var isActive, isQuotaExceeded bool
	var quotaResetTimeStr sql.NullString
	var lastUsedAtStr sql.NullString
	var createdAtStr, updatedAtStr sql.NullString

	err := row.Scan(&id, &service, &keyValue, &model, &isActive, &isQuotaExceeded, &quotaResetTimeStr, &rpmLimit, &tpmLimit, &rpdLimit, &requestMade, &lastUsedAtStr, &lastError, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}

	qrt := parseTimePtr(quotaResetTimeStr)
	lsu := parseTimePtr(lastUsedAtStr)
	ca := parseTime(createdAtStr)
	ua := parseTime(updatedAtStr)
	return entity.RestoreAPIKey(id, service, keyValue, model, isActive, isQuotaExceeded, qrt, rpmLimit, tpmLimit, rpdLimit, requestMade, lsu, lastError, ca, ua)
}
