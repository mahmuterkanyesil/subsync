package valueobject

import "subsync/internal/core/domain/exception"

type MediaInfo struct {
	MediaType     MediaType
	SeriesName    string
	SeasonNumber  int
	EpisodeNumber int
}

func NewMediaInfo(mediaType MediaType, seriesName string, seasonNumber int, episodeNumber int) (MediaInfo, error) {
	if !mediaType.IsValid() {
		return MediaInfo{}, &exception.InvalidMediaTypeException{Message: "invalid media type"}
	}
	if seriesName == "" && mediaType == MediaTypeSeries {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "series name cannot be empty for series type"}
	}
	if seasonNumber < 0 {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "season number cannot be negative"}
	}
	if episodeNumber < 0 {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "episode number cannot be negative"}
	}
	return MediaInfo{
		MediaType:     mediaType,
		SeriesName:    seriesName,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
	}, nil
}
