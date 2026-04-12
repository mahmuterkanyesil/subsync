package gin

import (
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
)

// SubtitleResponse, Subtitle entity'sinin REST katmanına açılan DTO'sudur.
type SubtitleResponse struct {
	ID            string `json:"id"`
	EngPath       string `json:"eng_path"`
	Status        string `json:"status"`
	MediaType     string `json:"media_type"`
	SeriesName    string `json:"series_name,omitempty"`
	SeasonNumber  int    `json:"season_number,omitempty"`
	EpisodeNumber int    `json:"episode_number,omitempty"`
	Embedded      bool   `json:"embedded"`
	LastError     string `json:"last_error,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// StatsResponse, SubtitleStats'ın REST DTO'sudur.
type StatsResponse struct {
	Total          int `json:"total"`
	Queued         int `json:"queued"`
	Done           int `json:"done"`
	Embedded       int `json:"embedded"`
	Error          int `json:"error"`
	QuotaExhausted int `json:"quota_exhausted"`
	EmbedFailed    int `json:"embed_failed"`
}

func toSubtitleResponse(s *entity.Subtitle) SubtitleResponse {
	mi := s.MediaInfo()
	return SubtitleResponse{
		ID:            s.ID().String(),
		EngPath:       s.EngPath(),
		Status:        string(s.Status()),
		MediaType:     string(mi.MediaType),
		SeriesName:    mi.SeriesName,
		SeasonNumber:  mi.SeasonNumber,
		EpisodeNumber: mi.EpisodeNumber,
		Embedded:      s.Embedded(),
		LastError:     s.LastError(),
		CreatedAt:     s.CreatedAt().Format("2006-01-02T15:04:05Z"),
		UpdatedAt:     s.UpdatedAt().Format("2006-01-02T15:04:05Z"),
	}
}

func toSubtitleResponses(subtitles []*entity.Subtitle) []SubtitleResponse {
	result := make([]SubtitleResponse, len(subtitles))
	for i, s := range subtitles {
		result[i] = toSubtitleResponse(s)
	}
	return result
}

func toStatsResponse(stats port.SubtitleStats) StatsResponse {
	return StatsResponse{
		Total:          stats.Total,
		Queued:         stats.Queued,
		Done:           stats.Done,
		Embedded:       stats.Embedded,
		Error:          stats.Error,
		QuotaExhausted: stats.QuotaExhausted,
		EmbedFailed:    stats.EmbedFailed,
	}
}
