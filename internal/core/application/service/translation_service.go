package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/event"
	"subsync/internal/core/domain/valueobject"
	domainservice "subsync/internal/core/domain/service"
	"subsync/pkg/srt"
	"time"
)

type TranslationService struct {
	subtitleRepo port.SubtitleRepository
	apiKeyRepo   port.APIKeyRepository
	translator   port.TranslationProvider
	progress     port.ProgressStore
	events       port.EventPublisher
	batchSize    int
}

func NewTranslationService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	translator port.TranslationProvider,
	progress port.ProgressStore,
	events port.EventPublisher,
	batchSize int,
) *TranslationService {
	return &TranslationService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		translator:   translator,
		progress:     progress,
		events:       events,
		batchSize:    batchSize,
	}
}

func (s *TranslationService) publish(e event.DomainEvent) {
	if s.events != nil {
		s.events.Publish(e)
	}
}

func (s *TranslationService) Translate(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}

	if subtitle.Status() == valueobject.StatusDone {
		return nil
	}

	content, err := os.ReadFile(engPath)
	if err != nil {
		return err
	}

	blocks := srt.Parse(string(content))

	translated, hasProgress, err := s.progress.Load(ctx, engPath)
	if err != nil {
		return err
	}
	if !hasProgress {
		translated = []port.SRTBlock{}
	}

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
			errStr := err.Error()
			switch {
			case strings.Contains(errStr, "quota_exhausted_rpm"):
				_ = s.progress.Save(ctx, engPath, translated)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(60 * time.Second):
				}
				continue

			case strings.Contains(errStr, "quota_exhausted_rpd"),
				strings.Contains(errStr, "quota_exhausted"):
				apiKey.MarkAsQuotaExceeded(time.Now().Add(24 * time.Hour))
				_ = s.apiKeyRepo.Save(ctx, apiKey)
				trPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + ".tr.srt"
				_ = os.Remove(trPath)
				retryCount++
				if retryCount >= maxRetry {
					_ = s.progress.Save(ctx, engPath, translated)
					_ = subtitle.TransitionTo(valueobject.StatusQuotaExhausted)
					_ = s.subtitleRepo.Save(ctx, subtitle)
					return fmt.Errorf("all api keys quota exhausted")
				}
				continue

			default:
				_ = s.progress.Save(ctx, engPath, translated)
				return err
			}
		}

		retryCount = 0
		apiKey.MarkAsUsed()
		_ = s.apiKeyRepo.Save(ctx, apiKey)
		translated = append(translated, result...)
		_ = s.progress.Save(ctx, engPath, translated)
		i += s.batchSize
	}

	// Domain service ile doğrula
	texts := make([]string, len(translated))
	for i, b := range translated {
		texts[i] = b.Text
	}
	if !domainservice.IsTranslatedToTurkish(texts) {
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return fmt.Errorf("translation validation failed: not turkish")
	}

	trPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + ".tr.srt"
	if err := os.WriteFile(trPath, []byte(srt.Format(translated)), 0644); err != nil {
		return err
	}

	_ = s.progress.Clear(ctx, engPath)

	if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
		return err
	}
	if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
		return err
	}

	s.publish(event.NewTranslationCompleted(engPath))
	return nil
}
