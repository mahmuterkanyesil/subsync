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
)

type TranslationService struct {
	subtitleRepo port.SubtitleRepository
	apiKeyRepo   port.APIKeyRepository
	translator   port.TranslationProvider
	progress     port.ProgressStore
}

func NewTranslationService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	translator port.TranslationProvider,
	progress port.ProgressStore,
) *TranslationService {
	return &TranslationService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		translator:   translator,
		progress:     progress,
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
	_ = len(blocks)

	// 5. Önceki progress var mı?
	translated, hasProgress, err := s.progress.Load(ctx, engPath)
	if err != nil {
		return err
	}
	if !hasProgress {
		translated = []port.SRTBlock{}
	}

	// 6. Kalan blokları çevir
	apiKey, err := s.apiKeyRepo.FindNextAvailable(ctx, "gemini")
	if err != nil {
		return err
	}

	remaining := blocks[len(translated):]
	for i := 0; i < len(remaining); i += 500 {
		end := min(i+500, len(remaining))
		batch := remaining[i:end]

		result, err := s.translator.TranslateBatch(ctx, batch, apiKey.KeyValue())
		if err != nil {
			// Progress'i kaydet, hata dön
			_ = s.progress.Save(ctx, engPath, translated)
			return err
		}

		translated = append(translated, result...)

		// 7. Progress kaydet
		if err := s.progress.Save(ctx, engPath, translated); err != nil {
			return err
		}
	}

	// 8. Validation
	if !srt.IsTurkish(translated) {
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return fmt.Errorf("translation validation failed: not turkish")
	}

	// 9. .tr.srt dosyasına yaz
	trPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + ".tr.srt"
	if err := os.WriteFile(trPath, []byte(srt.Format(translated)), 0644); err != nil {
		return err
	}

	// 10. Progress temizle
	_ = s.progress.Clear(ctx, engPath)

	// 11. Done olarak işaretle
	if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
		return err
	}
	return s.subtitleRepo.Save(ctx, subtitle)
}
