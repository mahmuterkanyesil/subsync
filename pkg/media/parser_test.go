package media_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/media"
)

func TestParseMediaInfo(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		wantType    valueobject.MediaType
		wantSeries  string
		wantSeason  int
		wantEpisode int
	}{
		// Movie paths (no /tv/ segment)
		{
			name:        "movie path returns Movie type",
			filePath:    "/movies/Inception.mkv",
			wantType:    valueobject.MediaTypeMovie,
			wantSeries:  "",
			wantSeason:  0,
			wantEpisode: 0,
		},
		{
			name:        "plain file without tv segment",
			filePath:    "/media/files/film.mkv",
			wantType:    valueobject.MediaTypeMovie,
			wantSeries:  "",
			wantSeason:  0,
			wantEpisode: 0,
		},

		// Series paths (with /tv/ segment)
		{
			name:        "series with SxxExx in filename",
			filePath:    "/media/tv/Breaking Bad/Season 3/S03E07.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Breaking Bad",
			wantSeason:  3,
			wantEpisode: 7,
		},
		{
			name:        "case-insensitive TV directory",
			filePath:    "/media/TV/Westworld/S01E01.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Westworld",
			wantSeason:  1,
			wantEpisode: 1,
		},
		{
			name:        "series with double-digit season and episode",
			filePath:    "/tv/The Wire/S12E24.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "The Wire",
			wantSeason:  12,
			wantEpisode: 24,
		},
		{
			name:        "series with SxxExx in srt filename",
			filePath:    "/tv/Show/Show.S04E10.eng.srt",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  4,
			wantEpisode: 10,
		},
		{
			name:        "SxxExx wins over season directory pattern",
			filePath:    "/tv/Show/Season 2/S02E05.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  2,
			wantEpisode: 5,
		},
		{
			name:        "series path without SxxExx — zeroes",
			filePath:    "/tv/Documentary/episode.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Documentary",
			wantSeason:  0,
			wantEpisode: 0,
		},
		{
			name:        "lowercase sxxexx pattern",
			filePath:    "/tv/Show/s03e07.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  3,
			wantEpisode: 7,
		},
		{
			name:        "series name from directory right after tv",
			filePath:    "/data/tv/My Show Name/S01E01.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "My Show Name",
			wantSeason:  1,
			wantEpisode: 1,
		},
		{
			name:        "windows path with drive letter",
			filePath:    `C:\media\tv\Show\S01E02.mkv`,
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  1,
			wantEpisode: 2,
		},
		{
			name:        "season directory pattern without SxxExx",
			filePath:    "/tv/Show/Season 3/episode.mkv",
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  3,
			wantEpisode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := media.ParseMediaInfo(tt.filePath)
			assert.Equal(t, tt.wantType, got.MediaType)
			assert.Equal(t, tt.wantSeries, got.SeriesName)
			assert.Equal(t, tt.wantSeason, got.SeasonNumber)
			assert.Equal(t, tt.wantEpisode, got.EpisodeNumber)
		})
	}
}
