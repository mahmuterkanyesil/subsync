package sqlite

import (
	"context"
	"database/sql"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
	"time"
)

type SQLiteSubtitleRepository struct {
	db *sql.DB
}

func NewSQLiteSubtitleRepository(db *sql.DB) *SQLiteSubtitleRepository {
	return &SQLiteSubtitleRepository{db: db}
}

func (r *SQLiteSubtitleRepository) Save(ctx context.Context, s *entity.Subtitle) error {
	query := `
		INSERT INTO subtitles (eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(eng_path) DO UPDATE SET
			status = excluded.status,
			last_error = excluded.last_error,
			embedded = excluded.embedded,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
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
	query := `SELECT eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles WHERE eng_path = ?`

	row := r.db.QueryRowContext(ctx, query, path)
	return scanSubtitle(row)
}

func (r *SQLiteSubtitleRepository) FindAll(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSubtitles(rows)
}

func (r *SQLiteSubtitleRepository) FindPendingEmbed(ctx context.Context) ([]*entity.Subtitle, error) {
	query := `SELECT eng_path, media_type, series_name, season_number, episode_number, status, last_error, embedded, created_at, updated_at FROM subtitles WHERE status = 'done' AND embedded = 0`

	rows, err := r.db.QueryContext(ctx, query)
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
		}
		stats.Total += count
	}
	return stats, nil
}

// --- yardımcı fonksiyonlar ---

func scanSubtitle(row *sql.Row) (*entity.Subtitle, error) {
	var engPath, mediaType, seriesName, status, lastError string
	var seasonNumber, episodeNumber int
	var embedded bool
	var createdAt, updatedAt time.Time

	err := row.Scan(&engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
	subtitle, err := entity.NewSubtitle(mediaInfo, engPath)
	if err != nil {
		return nil, err
	}
	_ = subtitle.TransitionTo(valueobject.SubtitleStatus(status))
	return subtitle, nil
}

func scanSubtitles(rows *sql.Rows) ([]*entity.Subtitle, error) {
	var result []*entity.Subtitle
	for rows.Next() {
		var engPath, mediaType, seriesName, status, lastError string
		var seasonNumber, episodeNumber int
		var embedded bool
		var createdAt, updatedAt time.Time

		err := rows.Scan(&engPath, &mediaType, &seriesName, &seasonNumber, &episodeNumber, &status, &lastError, &embedded, &createdAt, &updatedAt)
		if err != nil {
			continue
		}

		mediaInfo, _ := valueobject.NewMediaInfo(valueobject.MediaType(mediaType), seriesName, seasonNumber, episodeNumber)
		subtitle, err := entity.NewSubtitle(mediaInfo, engPath)
		if err != nil {
			continue
		}
		_ = subtitle.TransitionTo(valueobject.SubtitleStatus(status))
		result = append(result, subtitle)
	}
	return result, nil
}
