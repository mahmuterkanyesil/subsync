package media

import (
	"regexp"
	"strconv"
	"strings"
	"subsync/internal/core/domain/valueobject"
)

var (
	seasonPattern  = regexp.MustCompile(`(?i)season\s+(\d+)`)
	episodePattern = regexp.MustCompile(`(?i)S(\d+)E(\d+)`)
)

func ParseMediaInfo(filePath string) valueobject.MediaInfo {
	// Normalize any backslashes to forward slashes so parsing works
	// the same on non-Windows CI runners when given Windows paths.
	normalized := strings.ReplaceAll(filePath, "\\", "/")
	parts := strings.Split(normalized, "/")

	if !containsDir(parts, "tv") {
		return valueobject.MediaInfo{
			MediaType: valueobject.MediaTypeMovie,
		}
	}

	tvIndex := findIndex(parts, "tv")
	seriesName := ""
	if tvIndex+1 < len(parts) {
		seriesName = parts[tvIndex+1]
	}

	seasonNumber := 0
	episodeNumber := 0

	for _, part := range parts {
		if m := seasonPattern.FindStringSubmatch(part); m != nil {
			seasonNumber, _ = strconv.Atoi(m[1])
		}
		if m := episodePattern.FindStringSubmatch(part); m != nil {
			seasonNumber, _ = strconv.Atoi(m[1])
			episodeNumber, _ = strconv.Atoi(m[2])
		}
	}

	return valueobject.MediaInfo{
		MediaType:     valueobject.MediaTypeSeries,
		SeriesName:    seriesName,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
	}
}

func containsDir(parts []string, dir string) bool {
	for _, p := range parts {
		if strings.EqualFold(p, dir) {
			return true
		}
	}
	return false
}

func findIndex(parts []string, dir string) int {
	for i, p := range parts {
		if strings.EqualFold(p, dir) {
			return i
		}
	}
	return -1
}
