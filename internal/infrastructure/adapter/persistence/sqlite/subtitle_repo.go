package sqlite

import (
	"context"
	"database/sql"
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
		INSERT INTO subtitles (id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(eng_path) DO UPDATE SET
			id         = COALESCE(subtitles.id, excluded.id),
			status     = excluded.status,
			last_error = excluded.last_error,
			embedded   = excluded.embedded,
			updated_at = excluded.updated_at
	`
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
		s.CreatedAt(),
		s.UpdatedAt(),
	)
	return err
}

func (r *SQLiteSubtitleRepository) FindByPath(ctx context.Context, path string) (*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles WHERE eng_path = ?`
	row := r.db.QueryRowContext(ctx, query, path)
	return scanSubtitle(row)
}

func (r *SQLiteSubtitleRepository) FindAll(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles WHERE status = 'done' AND embedded = 0`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindBySxxExx(ctx context.Context, season, episode int) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at
		FROM subtitles
		WHERE season_number = ? AND episode_number = ? AND season_number > 0 AND episode_number > 0`
	rows, err := r.db.QueryContext(ctx, query, season, episode)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	query := `SELECT id, eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at
		FROM subtitles WHERE status = ?`
	rows, err := r.db.QueryContext(ctx, query, string(status))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) Statistics(ctx context.Context) (port.SubtitleStats, error) {
	query := `SELECT status, COUNT(*) FROM subtitles GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return port.SubtitleStats{}, err
	}
	defer rows.Close()

	stats := port.SubtitleStats{}
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

// --- yardımcı fonksiyonlar ---

func scanSubtitle(row *sql.Row) (*entity.Subtitle, error) {
	var idStr sql.NullString
	var engPath, mediaType, seriesName, status, lastError string
	var seasonNumber, episodeNumber int
	var embedded bool
	var createdAtStr, updatedAtStr sql.NullString

	err := row.Scan(&idStr, &engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}

	id := parseOrNewUUID(idStr)
	mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
	ca := parseTime(createdAtStr)
	ua := parseTime(updatedAtStr)
	return entity.RestoreSubtitle(id, mediaInfo, engPath, valueobject.SubtitleStatus(status), lastError, embedded, ca, ua)
}

func scanSubtitles(rows *sql.Rows) ([]*entity.Subtitle, error) {
	var result []*entity.Subtitle
	for rows.Next() {
		var idStr sql.NullString
		var engPath, mediaType, seriesName, status, lastError string
		var seasonNumber, episodeNumber int
		var embedded bool
		var createdAtStr, updatedAtStr sql.NullString

		if err := rows.Scan(&idStr, &engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &createdAtStr, &updatedAtStr); err != nil {
			continue
		}

		id := parseOrNewUUID(idStr)
		mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
		ca := parseTime(createdAtStr)
		ua := parseTime(updatedAtStr)
		subtitle, err := entity.RestoreSubtitle(id, mediaInfo, engPath, valueobject.SubtitleStatus(status), lastError, embedded, ca, ua)
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
