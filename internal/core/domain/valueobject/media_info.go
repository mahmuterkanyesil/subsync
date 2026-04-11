package valueobject

import "subsync/internal/core/domain/exception"

type MediaType string

const (
	MediaTypeSeries MediaType = "series"
	MediaTypeMovie  MediaType = "movie"
)

type MediaInfo struct {
	MediaType     MediaType
	SeriesName    string
	SeasonNumber  int
	EpisodeNumber int
}

func NewMediaInfo(mediaType MediaType, seriesName string, seasonNumber int, episodeNumber int) (MediaInfo, error) {
	if mediaType != MediaTypeSeries && mediaType != MediaTypeMovie {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "invalid media type"}
	}
}