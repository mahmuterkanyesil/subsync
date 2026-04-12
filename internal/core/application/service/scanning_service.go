package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

var sxxExxRegex = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,2})`)

type ScanningService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
	taskQueue      port.TaskQueue
	watchDirs      []string
}

func NewScanningService(
	subtitleRepo port.SubtitleRepository,
	videoProcessor port.VideoProcessor,
	taskQueue port.TaskQueue,
	watchDirs []string,
) *ScanningService {
	return &ScanningService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
		taskQueue:      taskQueue,
		watchDirs:      watchDirs,
	}
}

func extractSxxExx(path string) (season, episode int, ok bool) {
	m := sxxExxRegex.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return 0, 0, false
	}
	fmt.Sscanf(m[1], "%d", &season)
	fmt.Sscanf(m[2], "%d", &episode)
	return season, episode, true
}

// inferMediaInfo dosya adından MediaInfo oluşturur.
// SxxExx deseni varsa dizi, yoksa film olarak işaretler.
func inferMediaInfo(videoPath string) valueobject.MediaInfo {
	base := filepath.Base(videoPath)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))

	season, episode, ok := extractSxxExx(videoPath)
	if ok {
		// Dizi — series name: SxxExx'ten önceki kısım
		seriesName := sxxExxRegex.Split(nameWithoutExt, 2)[0]
		seriesName = strings.Trim(strings.ReplaceAll(seriesName, ".", " "), " -_")
		if seriesName == "" {
			seriesName = nameWithoutExt
		}
		mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeSeries, seriesName, season, episode)
		if err == nil {
			return mi
		}
	}

	// Film
	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	if err == nil {
		return mi
	}
	return valueobject.MediaInfo{}
}

func (s *ScanningService) Scan(ctx context.Context) error {
	for _, dir := range s.watchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			ext := filepath.Ext(path)
			if ext != ".mkv" && ext != ".mp4" {
				return nil
			}

			hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, path)
			if err != nil || hasTr {
				return nil
			}

			engPath, err := s.videoProcessor.EnsureEngSubtitle(ctx, path)
			if err != nil {
				return nil
			}

			// DB'de tam eşleşme var mı?
			existing, err := s.subtitleRepo.FindByPath(ctx, engPath)
			if err == nil && existing != nil {
				status := existing.Status()
				if status == valueobject.StatusQueued || status == valueobject.StatusDone {
					return nil
				}
			}

			// SxxExx ile fuzzy match — video relocated mi?
			if err != nil || existing == nil {
				if season, episode, ok := extractSxxExx(path); ok {
					candidates, dbErr := s.subtitleRepo.FindBySxxExx(ctx, season, episode)
					if dbErr == nil && len(candidates) > 0 {
						log.Printf("video relocated S%02dE%02d: %s", season, episode, path)
						return nil
					}
				}
			}

			mediaInfo := inferMediaInfo(path)

			subtitle, err := entity.NewSubtitle(mediaInfo, engPath)
			if err != nil {
				return nil
			}

			if err := s.taskQueue.Enqueue(ctx, "translate_srt", port.TranslateTask{
				EngPath:   engPath,
				VideoPath: path,
			}); err != nil {
				return nil
			}

			return s.subtitleRepo.Save(ctx, subtitle)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
