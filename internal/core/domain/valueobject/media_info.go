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
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "series name cannot be empty"}
	}
	if seasonNumber < 0 {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "invalid media type"}
	}
	if episodeNumber < 0 {
		return MediaInfo{}, &exception.InvalidMediaInfoException{Message: "series name cannot be empty"}

	}
	return MediaInfo{
		MediaType:     mediaType,
		SeriesName:    seriesName,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
	}, nil
}
