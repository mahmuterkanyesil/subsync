package valueobject

type MediaType string

const (
	MediaTypeSeries MediaType = "series"
	MediaTypeMovie  MediaType = "movie"
)

func (m MediaType) IsValid() bool {
	return m == MediaTypeSeries || m == MediaTypeMovie
}
