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
	"subsync/pkg/logger"
	"subsync/pkg/srt"
	"time"
)

type TranslationService struct {
	subtitleRepo    port.SubtitleRepository
	apiKeyRepo      port.APIKeyRepository
	translator      port.TranslationProvider
	progress        port.ProgressStore
	events          port.EventPublisher
	batchSize       int
	modelPriority   []string
	exhaustedModels map[string]time.Time
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
		modelPriority: []string{
			"gemini-3.1-flash-lite-preview",
			"gemini-2.5-flash",
			"gemini-2.5-flash-lite",
			"gemini-3-flash-preview",
		},
		exhaustedModels: make(map[string]time.Time),
	}
}

func (s *TranslationService) pickModel() string {
	now := time.Now()
	for _, m := range s.modelPriority {
		if t, ok := s.exhaustedModels[m]; !ok || now.After(t) {
			delete(s.exhaustedModels, m)
			return m
		}
	}
	return ""
}

func (s *TranslationService) earliestModelReset() time.Time {
	var earliest time.Time
	for _, t := range s.exhaustedModels {
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}

func (s *TranslationService) publish(e event.DomainEvent) {
	if s.events != nil {
		s.events.Publish(e)
	}
}

func (s *TranslationService) Translate(ctx context.Context, engPath string) error {
	// Reset any RPD quotas that have passed their reset time before attempting translation.
	_ = s.apiKeyRepo.ResetExpiredQuotas(ctx)

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
	name := filepath.Base(engPath)

	translated, hasProgress, err := s.progress.Load(ctx, engPath)
	if err != nil {
		return err
	}
	if !hasProgress {
		translated = []port.SRTBlock{}
	} else {
		logger.Info("translate resume: %s — %d/%d blocks done", name, len(translated), len(blocks))
	}

	remaining := blocks[len(translated):]
	totalBatches := (len(remaining) + s.batchSize - 1) / s.batchSize
	batchNum := 0

	logger.Info("translate start: %s — %d blocks, %d batches", name, len(blocks), totalBatches)

	for i := 0; i < len(remaining); {
		end := i + s.batchSize
		if end > len(remaining) {
			end = len(remaining)
		}
		batch := remaining[i:end]

		currentModel := s.pickModel()
		if currentModel == "" {
			resetAt := s.earliestModelReset()
			wait := time.Until(resetAt) + 30*time.Second
			if wait < 0 {
				wait = 30 * time.Second
			}
			logger.Warn("translate: all models exhausted for %s — waiting %s", name, wait.Round(time.Second))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}

		apiKey, err := s.apiKeyRepo.FindNextAvailable(ctx, "gemini")
		if err != nil {
			_ = subtitle.TransitionTo(valueobject.StatusQuotaExhausted)
			_ = s.subtitleRepo.Save(ctx, subtitle)
			return fmt.Errorf("no active api keys configured for service gemini")
		}

		batchNum++
		logger.Info("translate batch %d/%d: %s [model=%s key=%d]", batchNum, totalBatches, name, currentModel, apiKey.ID())

		result, err := s.translator.TranslateBatch(ctx, batch, apiKey.KeyValue(), currentModel)
		if err != nil {
			errStr := err.Error()
			switch {
			case strings.Contains(errStr, "quota_exhausted_rpm"):
				logger.Warn("translate: RPM quota hit for %s [model=%s] — waiting 60s", name, currentModel)
				_ = s.progress.Save(ctx, engPath, translated)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(60 * time.Second):
				}
				continue

			case strings.Contains(errStr, "quota_exhausted_rpd"),
				strings.Contains(errStr, "quota_exhausted"):
				logger.Warn("translate: RPD quota hit for %s [model=%s] — switching model", name, currentModel)
				s.exhaustedModels[currentModel] = time.Now().Add(24 * time.Hour)
				trPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + ".tr.srt"
				_ = os.Remove(trPath)
				batchNum--
				continue

			default:
				logger.Error("translate error for %s: %v", name, err)
				_ = s.progress.Save(ctx, engPath, translated)
				return err
			}
		}

		apiKey.MarkAsUsed()
		_ = s.apiKeyRepo.Save(ctx, apiKey)
		_ = s.apiKeyRepo.IncrementModelUsage(ctx, apiKey.ID(), currentModel)
		translated = append(translated, result...)
		_ = s.progress.Save(ctx, engPath, translated)
		i += s.batchSize

		// Rate limit delay between batches
		if i < len(remaining) {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(10 * time.Second):
			}
		}
	}

	// SRT block structure validation
	if err := domainservice.ValidateTranslation(blocks, translated); err != nil {
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return fmt.Errorf("srt validation failed: %w", err)
	}

	// Domain service ile doğrula
	texts := make([]string, len(translated))
	for i := range translated {
		texts[i] = translated[i].Text
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

	if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
		return err
	}
	if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
		return err
	}
	// Clear progress only after DB is committed so a retry can resume from
	// the saved blocks instead of restarting from scratch.
	_ = s.progress.Clear(ctx, engPath)

	logger.Info("translate done: %s — %d blocks", name, len(translated))
	s.publish(event.NewTranslationCompleted(engPath))
	return nil
}
