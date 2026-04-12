package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/srt"
	"time"
)

type TranslationService struct {
	subtitleRepo port.SubtitleRepository
	apiKeyRepo   port.APIKeyRepository
	translator   port.TranslationProvider
	progress     port.ProgressStore
	batchSize    int
}

func NewTranslationService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	translator port.TranslationProvider,
	progress port.ProgressStore,
	batchSize int,
) *TranslationService {
	return &TranslationService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		translator:   translator,
		progress:     progress,
		batchSize:    batchSize,
	}
}

func (s *TranslationService) Translate(ctx context.Context, engPath string) error {
	// 1. Subtitle'ı DB'den getir
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}

	// 2. Zaten done ise geç
	if subtitle.Status() == valueobject.StatusDone {
		return nil
	}

	// 3. İngilizce SRT dosyasını oku
	content, err := os.ReadFile(engPath)
	if err != nil {
		return err
	}

	// 4. Bloklara böl
	blocks := srt.Parse(string(content))

	// 5. Önceki progress var mı?
	translated, hasProgress, err := s.progress.Load(ctx, engPath)
	if err != nil {
		return err
	}
	if !hasProgress {
		translated = []port.SRTBlock{}
	}

	// 6. Kalan blokları çevir — key rotation ile
	remaining := blocks[len(translated):]
	retryCount := 0
	maxRetry := 3

	for i := 0; i < len(remaining); {
		end := i + s.batchSize
		if end > len(remaining) {
			end = len(remaining)
		}
		batch := remaining[i:end]

		apiKey, err := s.apiKeyRepo.FindNextAvailable(ctx, "gemini")
		if err != nil {
			_ = s.progress.Save(ctx, engPath, translated)
			return fmt.Errorf("no available api key: %w", err)
		}

		result, err := s.translator.TranslateBatch(ctx, batch, apiKey.KeyValue())
		if err != nil {
			if strings.Contains(err.Error(), "quota_exhausted") {
				apiKey.MarkAsQuotaExceeded(time.Now().Add(24 * time.Hour))
				_ = s.apiKeyRepo.Save(ctx, apiKey)

				retryCount++
				if retryCount >= maxRetry {
					_ = s.progress.Save(ctx, engPath, translated)
					return fmt.Errorf("all api keys quota exhausted")
				}
				continue // i artmaz, aynı batch tekrar dener
			}
			_ = s.progress.Save(ctx, engPath, translated)
			return err
		}

		retryCount = 0
		apiKey.MarkAsUsed()
		_ = s.apiKeyRepo.Save(ctx, apiKey)

		translated = append(translated, result...)
		_ = s.progress.Save(ctx, engPath, translated)
		i += s.batchSize
	}

	// 7. Validation
	if !srt.IsTurkish(translated) {
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return fmt.Errorf("translation validation failed: not turkish")
	}

	// 8. .tr.srt dosyasına yaz
	trPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + ".tr.srt"
	if err := os.WriteFile(trPath, []byte(srt.Format(translated)), 0644); err != nil {
		return err
	}

	// 9. Progress temizle
	_ = s.progress.Clear(ctx, engPath)

	// 10. Done olarak işaretle
	if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
		return err
	}
	return s.subtitleRepo.Save(ctx, subtitle)
}
