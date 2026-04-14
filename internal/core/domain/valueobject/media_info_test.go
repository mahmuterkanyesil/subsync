package valueobject_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/valueobject"
)

func TestNewMediaInfo_Valid(t *testing.T) {
	tests := []struct {
		name          string
		mediaType     valueobject.MediaType
		seriesName    string
		season        int
		episode       int
		wantType      valueobject.MediaType
		wantSeries    string
		wantSeason    int
		wantEpisode   int
	}{
		{
			name:        "movie with all zeroes",
			mediaType:   valueobject.MediaTypeMovie,
			seriesName:  "",
			season:      0,
			episode:     0,
			wantType:    valueobject.MediaTypeMovie,
			wantSeries:  "",
			wantSeason:  0,
			wantEpisode: 0,
		},
		{
			name:        "series with full data",
			mediaType:   valueobject.MediaTypeSeries,
			seriesName:  "Breaking Bad",
			season:      3,
			episode:     7,
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Breaking Bad",
			wantSeason:  3,
			wantEpisode: 7,
		},
		{
			name:        "series with zero season and episode",
			mediaType:   valueobject.MediaTypeSeries,
			seriesName:  "Show",
			season:      0,
			episode:     0,
			wantType:    valueobject.MediaTypeSeries,
			wantSeries:  "Show",
			wantSeason:  0,
			wantEpisode: 0,
		},
		{
			name:        "movie with non-empty series name allowed",
			mediaType:   valueobject.MediaTypeMovie,
			seriesName:  "ignored",
			season:      2,
			episode:     5,
			wantType:    valueobject.MediaTypeMovie,
			wantSeries:  "ignored",
			wantSeason:  2,
			wantEpisode: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi, err := valueobject.NewMediaInfo(tt.mediaType, tt.seriesName, tt.season, tt.episode)
			require.NoError(t, err)
			assert.Equal(t, tt.wantType, mi.MediaType)
			assert.Equal(t, tt.wantSeries, mi.SeriesName)
			assert.Equal(t, tt.wantSeason, mi.SeasonNumber)
			assert.Equal(t, tt.wantEpisode, mi.EpisodeNumber)
		})
	}
}

func TestNewMediaInfo_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		mediaType  valueobject.MediaType
		seriesName string
		season     int
		episode    int
	}{
		{
			name:      "invalid media type",
			mediaType: valueobject.MediaType("documentary"),
		},
		{
			name:      "series with empty name",
			mediaType: valueobject.MediaTypeSeries,
			// seriesName defaults to ""
		},
		{
			name:       "negative season number",
			mediaType:  valueobject.MediaTypeSeries,
			seriesName: "Show",
			season:     -1,
			episode:    0,
		},
		{
			name:       "negative episode number",
			mediaType:  valueobject.MediaTypeSeries,
			seriesName: "Show",
			season:     0,
			episode:    -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := valueobject.NewMediaInfo(tt.mediaType, tt.seriesName, tt.season, tt.episode)
			assert.Error(t, err)
		})
	}
}
