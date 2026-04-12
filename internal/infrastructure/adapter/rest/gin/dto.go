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

type APIKeyResponse struct {
	ID              int    `json:"id"`
	Service         string `json:"service"`
	IsActive        bool   `json:"is_active"`
	IsQuotaExceeded bool   `json:"is_quota_exceeded"`
	QuotaResetTime  string `json:"quota_reset_time,omitempty"`
	RequestMade     int    `json:"request_made"`
	LastUsedAt      string `json:"last_used_at,omitempty"`
	LastError       string `json:"last_error,omitempty"`
	CreatedAt       string `json:"created_at"`
}

func toAPIKeyResponse(k *entity.APIKey) APIKeyResponse {
	r := APIKeyResponse{
		ID:              k.ID(),
		Service:         k.Service(),
		IsActive:        k.IsActive(),
		IsQuotaExceeded: k.IsQuotaExceeded(),
		RequestMade:     k.RequestMade(),
		LastError:       k.LastError(),
		CreatedAt:       k.CreatedAt().Format("2006-01-02 15:04"),
	}
	if k.QuotaResetTime() != nil {
		r.QuotaResetTime = k.QuotaResetTime().Format("2006-01-02 15:04")
	}
	if k.LastUsedAt() != nil {
		r.LastUsedAt = k.LastUsedAt().Format("2006-01-02 15:04")
	}
	return r
}

func toAPIKeyResponses(keys []*entity.APIKey) []APIKeyResponse {
	result := make([]APIKeyResponse, len(keys))
	for i, k := range keys {
		result[i] = toAPIKeyResponse(k)
	}
	return result
}

type WatchDirResponse struct {
	ID        int    `json:"id"`
	Path      string `json:"path"`
	IsEnabled bool   `json:"is_enabled"`
	CreatedAt string `json:"created_at"`
}

func toWatchDirResponses(dirs []*entity.WatchDir) []WatchDirResponse {
	result := make([]WatchDirResponse, len(dirs))
	for i, d := range dirs {
		result[i] = WatchDirResponse{
			ID:        d.ID(),
			Path:      d.Path(),
			IsEnabled: d.IsEnabled(),
			CreatedAt: d.CreatedAt().Format("2006-01-02 15:04"),
		}
	}
	return result
}

type SettingsData struct {
	CurrentPage string
	WatchDirs   []WatchDirResponse
	Flash       string
	FlashOK     bool
}

type DashboardData struct {
	CurrentPage string
	Stats       StatsResponse
}

type RecordsData struct {
	CurrentPage string
	Records     []SubtitleResponse
	Filter      string
	Total       int
}

type KeysData struct {
	CurrentPage string
	Keys        []APIKeyResponse
	Flash       string
	FlashOK     bool
}
