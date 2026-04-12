package service

import (
	"context"
	"os"
	"path/filepath"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

type ScanningService struct {
	subtitleRepo   port.SubtitleRepository
	videoProcessor port.VideoProcessor
	taskQueue      port.TaskQueue
	watchDirs      []string
}

func NewScanningService(subtitleRepo port.SubtitleRepository, videoProcessor port.VideoProcessor, taskQueue port.TaskQueue, watchDirs []string) *ScanningService {
	return &ScanningService{
		subtitleRepo:   subtitleRepo,
		videoProcessor: videoProcessor,
		taskQueue:      taskQueue,
		watchDirs:      watchDirs,
	}
}

func (s *ScanningService) Scan(ctx context.Context) error {
	for _, dir := range s.watchDirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			// 1. Video dosyası mı?
			ext := filepath.Ext(path)
			if ext != ".mkv" && ext != ".mp4" {
				return nil
			}

			// 2. Türkçe subtitle var mı?
			hasTr, err := s.videoProcessor.HasTurkishSubtitle(ctx, path)
			if err != nil {
				return nil
			}
			if hasTr {
				return nil
			}

			// 3. İngilizce subtitle bul veya çıkar
			engPath, err := s.videoProcessor.EnsureEngSubtitle(ctx, path)
			if err != nil {
				return nil
			}

			// 4. DB'de kayıt var mı?
			existing, err := s.subtitleRepo.FindByPath(ctx, engPath)
			if err == nil && existing != nil {
				status := existing.Status()
				if status == valueobject.StatusQueued || status == valueobject.StatusDone {
					return nil
				}
			}

			// 5. MediaInfo oluştur
			mediaInfo := valueobject.MediaInfo{}

			// 6. Subtitle entity oluştur
			subtitle, err := entity.NewSubtitle(mediaInfo, engPath)
			if err != nil {
				return nil
			}

			// 7. Kuyruğa ekle
			err = s.taskQueue.Enqueue(ctx, "translate_srt", map[string]string{
				"eng_path":   engPath,
				"video_path": path,
			})
			if err != nil {
				return nil
			}

			// 8. DB'ye kaydet
			return s.subtitleRepo.Save(ctx, subtitle)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
