package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/logger"
)

var sxxExxRegex = regexp.MustCompile(`[Ss](\d{1,2})[Ee](\d{1,2})`)

type ScanningService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
	taskQueue      port.TaskQueue
	watchDirs      []string           // fallback from env config
	watchDirRepo   port.WatchDirRepository // optional: overrides watchDirs when non-empty
}

func NewScanningService(
	subtitleRepo port.SubtitleRepository,
	videoProcessor port.VideoProcessor,
	taskQueue port.TaskQueue,
	watchDirs []string,
	watchDirRepo port.WatchDirRepository,
) *ScanningService {
	return &ScanningService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
		taskQueue:      taskQueue,
		watchDirs:      watchDirs,
		watchDirRepo:   watchDirRepo,
	}
}

func (s *ScanningService) resolveWatchDirs(ctx context.Context) []string {
	if s.watchDirRepo != nil {
		if dbDirs, err := s.watchDirRepo.FindEnabled(ctx); err == nil && len(dbDirs) > 0 {
			return dbDirs
		}
	}
	return s.watchDirs
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
func inferMediaInfo(videoPath string) *valueobject.MediaInfo {
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
	return &valueobject.MediaInfo{}
}

func (s *ScanningService) Scan(ctx context.Context) error {
	for _, dir := range s.resolveWatchDirs(ctx) {
		logger.Debug("scanning directory: %s", dir)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				if err != nil {
					logger.Warn("walk error for %s: %v", path, err)
				}
				return nil
			}

			ext := filepath.Ext(path)
			if ext != ".mkv" && ext != ".mp4" {
				logger.Debug("skip non-video: %s", path)
				return nil
			}

			hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, path)
			if err != nil {
				logger.Warn("ffprobe check failed: %s — %v", filepath.Base(path), err)
				return nil
			}
			if hasTr {
				logger.Info("skip (has TR sub): %s", filepath.Base(path))
				return nil
			}

			engPath, err := s.videoProcessor.EnsureEngSubtitle(ctx, path)
			if err != nil {
				logger.Warn("sub extract failed: %s — %v", filepath.Base(path), err)
				return nil
			}

			// DB'de tam eşleşme var mı?
			existing, err := s.subtitleRepo.FindByPath(ctx, engPath)
			if err == nil && existing != nil {
				status := existing.Status()
				switch status {
				case valueobject.StatusQueued,
					valueobject.StatusDone,
					valueobject.StatusEmbedded,
					valueobject.StatusQuotaExhausted:
					logger.Debug("existing subtitle status %s for %s: skip", string(status), engPath)
					return nil
				}
			}

			// SxxExx ile fuzzy match — video relocated mi?
			if err != nil || existing == nil {
				if season, episode, ok := extractSxxExx(path); ok {
					candidates, dbErr := s.subtitleRepo.FindBySxxExx(ctx, season, episode)
					if dbErr == nil && len(candidates) > 0 {
						logger.Info("video relocated S%02dE%02d: %s", season, episode, filepath.Base(path))
						return nil
					}
				}
			}

			mediaInfo := inferMediaInfo(path)

			subtitle, err := entity.NewSubtitle(mediaInfo, engPath)
			if err != nil {
				logger.Error("subtitle entity failed: %s — %v", filepath.Base(engPath), err)
				return nil
			}

			if err := s.taskQueue.Enqueue(ctx, "translate_srt", port.TranslateTask{
				EngPath:   engPath,
				VideoPath: path,
			}); err != nil {
				logger.Error("enqueue failed: %s — %v", filepath.Base(engPath), err)
				return nil
			}

			logger.Info("queued translate: %s", filepath.Base(engPath))

			if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
				logger.Error("subtitle save failed for %s: %v", engPath, err)
				return nil
			}
			logger.Debug("subtitle saved: %s", engPath)
			return nil
		})
		if err != nil {
			return err
		}
	}
	s.cleanStaleRecords(ctx)
	return nil
}

func (s *ScanningService) cleanStaleRecords(ctx context.Context) {
	all, err := s.subtitleRepo.FindAll(ctx)
	if err != nil {
		return
	}
	for _, sub := range all {
		if _, statErr := os.Stat(sub.EngPath()); os.IsNotExist(statErr) {
			if delErr := s.subtitleRepo.Delete(ctx, sub.EngPath()); delErr == nil {
				logger.Info("stale record removed: %s", filepath.Base(sub.EngPath()))
			}
		}
	}
}
