package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"

	"github.com/google/uuid"
)

type SQLiteSubtitleRepository struct {
	db *sql.DB
}

func NewSQLiteSubtitleRepository(db *sql.DB) *SQLiteSubtitleRepository {
	return &SQLiteSubtitleRepository{db: db}
}

func (r *SQLiteSubtitleRepository) Save(ctx context.Context, s *entity.Subtitle) error {
	query := `
		INSERT INTO subtitles (id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, retry_count, last_retry_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(eng_path) DO UPDATE SET
			id           = COALESCE(subtitles.id, excluded.id),
			status       = excluded.status,
			last_error   = excluded.last_error,
			embedded     = excluded.embedded,
			retry_count  = excluded.retry_count,
			last_retry_at= excluded.last_retry_at,
			updated_at   = excluded.updated_at
	`
	var lastRetryAt interface{}
	if s.LastRetryAt() != nil {
		lastRetryAt = *s.LastRetryAt()
	}
	_, err := r.db.ExecContext(ctx, query,
		s.ID().String(),
		s.EngPath(),
		s.MediaInfo().MediaType,
		s.MediaInfo().SeriesName,
		s.MediaInfo().SeasonNumber,
		s.MediaInfo().EpisodeNumber,
		s.Status(),
		s.LastError(),
		s.Embedded(),
		s.RetryCount(),
		lastRetryAt,
		s.CreatedAt(),
		s.UpdatedAt(),
	)
	return err
}

func (r *SQLiteSubtitleRepository) FindByPath(ctx context.Context, path string) (*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, retry_count, last_retry_at, created_at, updated_at FROM subtitles WHERE eng_path = ?`
	row := r.db.QueryRowContext(ctx, query, path)
	return scanSubtitle(row)
}

func (r *SQLiteSubtitleRepository) FindAll(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, retry_count, last_retry_at, created_at, updated_at FROM subtitles ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, retry_count, last_retry_at, created_at, updated_at FROM subtitles WHERE status = 'done' AND embedded = 0`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at
		FROM subtitles WHERE status = ? ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) Statistics(ctx context.Context) (*port.SubtitleStats, error) {
	query := `SELECT status, COUNT(*) FROM subtitles GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := &port.SubtitleStats{}
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		switch status {
		case "done":
			stats.Done = count
		case "queued":
			stats.Queued = count
		case "error":
			stats.Error = count
		case "quota_exhausted":
			stats.QuotaExhausted = count
		case "embedded":
			stats.Embedded = count
		case "embed_failed":
			stats.EmbedFailed = count
		}
		stats.Total += count
	}
	return stats, nil
}

func (r *SQLiteSubtitleRepository) Delete(ctx context.Context, engPath string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM subtitles WHERE eng_path = ?", engPath)
	return err
}

func (r *SQLiteSubtitleRepository) DeleteMany(ctx context.Context, engPaths []string) error {
	if len(engPaths) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(engPaths))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(engPaths))
	for i, p := range engPaths {
		args[i] = p
	}
	_, err := r.db.ExecContext(ctx, fmt.Sprintf("DELETE FROM subtitles WHERE eng_path IN (%s)", placeholders), args...)
	return err
}

var validSortCols = map[string]string{
	"created_at":  "created_at",
	"updated_at":  "updated_at",
	"status":      "status",
	"series_name": "series_name",
	"eng_path":    "eng_path",
}

func (r *SQLiteSubtitleRepository) FindWithFilters(ctx context.Context, f port.SubtitleFilter) (*port.SubtitlePage, error) {
	sortCol, ok := validSortCols[f.SortBy]
	if !ok {
		sortCol = "created_at"
	}
	order := "DESC"
	if strings.ToUpper(f.Order) == "ASC" {
		order = "ASC"
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}

	var where []string
	var args []any

	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(f.Status))
	}
	if f.Query != "" {
		like := "%" + f.Query + "%"
		where = append(where, "(LOWER(eng_path) LIKE LOWER(?) OR LOWER(series_name) LIKE LOWER(?))")
		args = append(args, like, like)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM subtitles %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, err
	}

	dataQuery := fmt.Sprintf(
		`SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, retry_count, last_retry_at, created_at, updated_at
		 FROM subtitles %s ORDER BY %s %s LIMIT ? OFFSET ?`,
		whereClause, sortCol, order,
	)
	dataArgs := append(args, f.Limit, f.Offset)
	rows, err := r.db.QueryContext(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items, err := scanSubtitles(rows)
	if err != nil {
		return nil, err
	}
	return &port.SubtitlePage{Items: items, Total: total}, nil
}

// --- yardımcı fonksiyonlar ---

func scanSubtitle(row *sql.Row) (*entity.Subtitle, error) {
	var idStr sql.NullString
	var engPath, mediaType, seriesName, status, lastError string
	var seasonNumber, episodeNumber, retryCount int
	var embedded bool
	var lastRetryAtStr, createdAtStr, updatedAtStr sql.NullString

	err := row.Scan(&idStr, &engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &retryCount, &lastRetryAtStr, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}

	id := parseOrNewUUID(idStr)
	mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
	ca := parseTime(createdAtStr)
	ua := parseTime(updatedAtStr)
	lra := parseTimePtr(lastRetryAtStr)
	return entity.RestoreSubtitleFull(id, mediaInfo, engPath, valueobject.SubtitleStatus(status), lastError, embedded, retryCount, lra, ca, ua)
}

func scanSubtitles(rows *sql.Rows) ([]*entity.Subtitle, error) {
	var result []*entity.Subtitle
	for rows.Next() {
		var idStr sql.NullString
		var engPath, mediaType, seriesName, status, lastError string
		var seasonNumber, episodeNumber, retryCount int
		var embedded bool
		var lastRetryAtStr, createdAtStr, updatedAtStr sql.NullString

		if err := rows.Scan(&idStr, &engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &retryCount, &lastRetryAtStr, &createdAtStr, &updatedAtStr); err != nil {
			continue
		}

		id := parseOrNewUUID(idStr)
		mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
		ca := parseTime(createdAtStr)
		ua := parseTime(updatedAtStr)
		lra := parseTimePtr(lastRetryAtStr)
		subtitle, err := entity.RestoreSubtitleFull(id, mediaInfo, engPath, valueobject.SubtitleStatus(status), lastError, embedded, retryCount, lra, ca, ua)
		if err != nil {
			continue
		}
		result = append(result, subtitle)
	}
	return result, nil
}

func parseOrNewUUID(s sql.NullString) uuid.UUID {
	if s.Valid && s.String != "" {
		if id, err := uuid.Parse(s.String); err == nil {
			return id
		}
	}
	return uuid.New()
}
