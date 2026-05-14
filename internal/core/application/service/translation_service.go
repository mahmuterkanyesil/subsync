package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	settingsRepo    port.AppSettingsRepository
	batchSize       int
	mu              sync.Mutex // protects modelPriority and exhaustedModels
	modelPriority   []string
	exhaustedModels map[string]time.Time
	loadOnce        sync.Once
}

func NewTranslationService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	translator port.TranslationProvider,
	progress port.ProgressStore,
	events port.EventPublisher,
	batchSize int,
	settingsRepo port.AppSettingsRepository,
) *TranslationService {
	return &TranslationService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		translator:   translator,
		progress:     progress,
		events:       events,
		settingsRepo: settingsRepo,
		batchSize:    batchSize,
		modelPriority: append([]string(nil), defaultModelPriority...),
		exhaustedModels: make(map[string]time.Time),
	}
}

func (s *TranslationService) loadModelPriority(ctx context.Context) {
	if s.settingsRepo == nil {
		return
	}
	val, err := s.settingsRepo.GetSetting(ctx, "model_priority")
	if err != nil || val == "" {
		return
	}
	var models []string
	if err := json.Unmarshal([]byte(val), &models); err != nil || len(models) == 0 {
		return
	}
	s.mu.Lock()
	s.modelPriority = models
	s.mu.Unlock()
}

func (s *TranslationService) resolveModel(ctx context.Context, keyModel string) string {
	if keyModel == "" {
		return s.pickModel(ctx)
	}
	s.mu.Lock()
	now := time.Now()
	t, wasExhausted := s.exhaustedModels[keyModel]
	keyAvailable := !wasExhausted || now.After(t)
	if keyAvailable && wasExhausted {
		delete(s.exhaustedModels, keyModel)
	}
	s.mu.Unlock()
	if keyAvailable {
		if wasExhausted && s.settingsRepo != nil {
			_ = s.settingsRepo.SetSetting(ctx, "model_exhausted_"+keyModel, "")
		}
		return keyModel
	}
	return s.pickModel(ctx)
}

func (s *TranslationService) loadExhaustedModels(ctx context.Context) {
	s.loadOnce.Do(func() {
		if s.settingsRepo == nil {
			return
		}
		s.mu.Lock()
		models := make([]string, len(s.modelPriority))
		copy(models, s.modelPriority)
		s.mu.Unlock()

		for _, model := range models {
			val, err := s.settingsRepo.GetSetting(ctx, "model_exhausted_"+model)
			if err != nil || val == "" {
				continue
			}
			resetAt, err := time.Parse(time.RFC3339, val)
			if err != nil || time.Now().After(resetAt) {
				_ = s.settingsRepo.SetSetting(ctx, "model_exhausted_"+model, "")
				continue
			}
			s.mu.Lock()
			s.exhaustedModels[model] = resetAt
			s.mu.Unlock()
			logger.Info("translate: loaded exhausted model from DB [model=%s reset=%s]", model, resetAt.Format(time.RFC3339))
		}
	})
}

func (s *TranslationService) markModelExhausted(ctx context.Context, model string, resetAt time.Time) {
	s.mu.Lock()
	s.exhaustedModels[model] = resetAt
	s.mu.Unlock()
	if s.settingsRepo != nil {
		_ = s.settingsRepo.SetSetting(ctx, "model_exhausted_"+model, resetAt.Format(time.RFC3339))
	}
}

func (s *TranslationService) pickModel(ctx context.Context) string {
	s.mu.Lock()
	now := time.Now()
	var chosen, toClear string
	for _, m := range s.modelPriority {
		if t, ok := s.exhaustedModels[m]; !ok || now.After(t) {
			chosen = m
			if ok {
				toClear = m
				delete(s.exhaustedModels, m)
			}
			break
		}
	}
	s.mu.Unlock()
	if toClear != "" && s.settingsRepo != nil {
		_ = s.settingsRepo.SetSetting(ctx, "model_exhausted_"+toClear, "")
	}
	return chosen
}

func (s *TranslationService) earliestModelReset() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *TranslationService) Translate(ctx context.Context, engPath, targetLang string) error {
	if targetLang == "" {
		targetLang = "tr"
	}
	lang, ok := valueobject.LookupLanguage(targetLang)
	if !ok {
		lang = valueobject.DefaultLanguage()
	}

	s.loadExhaustedModels(ctx)
	s.loadModelPriority(ctx)

	// Reset any RPD quotas that have passed their reset time before attempting translation.
	_ = s.apiKeyRepo.ResetExpiredQuotas(ctx)

	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}

	if subtitle.Status() == valueobject.StatusDone {
		return nil
	}

	// If translated srt already exists, translation completed in a prior run but the
	// DB save failed (e.g. SQLITE_BUSY). Recover by updating status only.
	basePath := strings.TrimSuffix(engPath, ".eng.srt")
	basePath = strings.TrimSuffix(basePath, ".srt")
	trPath := basePath + "." + lang.Code + ".srt"
	oldTrPath := strings.TrimSuffix(engPath, filepath.Ext(engPath)) + "." + lang.Code + ".srt"

	if stat, err := os.Stat(trPath); err == nil && stat.Size() > 0 {
		// Found new format
	} else if stat, err := os.Stat(oldTrPath); err == nil && stat.Size() > 0 {
		// Found old format, use it for recovery
		trPath = oldTrPath
	}

	if _, statErr := os.Stat(trPath); statErr == nil {
		if transErr := subtitle.TransitionTo(valueobject.StatusDone); transErr != nil {
			logger.Warn("translate recovery: status transition failed for %s: %v", filepath.Base(engPath), transErr)
			return transErr
		}
		if saveErr := s.subtitleRepo.Save(ctx, subtitle); saveErr != nil {
			logger.Warn("translate recovery: db save failed for %s: %v", filepath.Base(engPath), saveErr)
			return saveErr
		}
		_ = s.progress.Clear(ctx, engPath)
		logger.Info("translate recovered: %s", filepath.Base(engPath))
		s.publish(event.NewTranslationCompleted(engPath))
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

	if len(translated) > len(blocks) {
		logger.Warn("translate: translated blocks (%d) > original blocks (%d) for %s — truncating", len(translated), len(blocks), name)
		translated = translated[:len(blocks)]
	}
	remaining := blocks[len(translated):]
	totalBatches := (len(remaining) + s.batchSize - 1) / s.batchSize
	batchNum := 0

	logger.Info("translate start: %s — %d blocks, %d batches [lang=%s]", name, len(blocks), totalBatches, targetLang)

	for i := 0; i < len(remaining); {
		end := i + s.batchSize
		if end > len(remaining) {
			end = len(remaining)
		}
		batch := remaining[i:end]

		apiKey, err := s.apiKeyRepo.FindNextAvailable(ctx, "gemini")
		if err != nil {
			subtitle.MarkError(fmt.Errorf("no active api keys for gemini"))
			_ = subtitle.TransitionTo(valueobject.StatusQuotaExhausted)
			_ = s.subtitleRepo.Save(ctx, subtitle)
			return fmt.Errorf("no active api keys configured for service gemini")
		}

		currentModel := s.resolveModel(ctx, apiKey.Model())
		if currentModel == "" {
			resetAt := s.earliestModelReset()
			logger.Warn("translate: all models exhausted for %s — next reset at %s", name, resetAt.Format(time.RFC3339))
			subtitle.MarkError(fmt.Errorf("all models exhausted, wait until %s", resetAt.Format(time.RFC3339)))
			_ = subtitle.TransitionTo(valueobject.StatusQuotaExhausted)
			_ = s.subtitleRepo.Save(ctx, subtitle)
			return fmt.Errorf("all models exhausted")
		}

		batchNum++
		logger.Info("translate batch %d/%d: %s [model=%s key=%d lang=%s]", batchNum, totalBatches, name, currentModel, apiKey.ID(), targetLang)

		result, err := s.translator.TranslateBatch(ctx, batch, apiKey.KeyValue(), currentModel, targetLang)
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
				s.markModelExhausted(ctx, currentModel, time.Now().Add(24*time.Hour))
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
		validationErr := fmt.Errorf("srt validation failed: %w", err)
		// Progress is corrupt or stale — clear it so the next retry translates
		// from scratch instead of re-loading the same invalid blocks forever.
		if len(remaining) == 0 {
			_ = s.progress.Clear(ctx, engPath)
			logger.Warn("translate: validation failed with full progress, clearing cache: %s — %v", name, err)
		}
		subtitle.MarkError(validationErr)
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return validationErr
	}

	texts := make([]string, len(translated))
	for i := range translated {
		texts[i] = translated[i].Text
	}
	if !domainservice.IsTranslatedToLanguage(texts, targetLang) {
		notLangErr := fmt.Errorf("translation validation failed: not %s", lang.NameEN)
		if len(remaining) == 0 {
			_ = s.progress.Clear(ctx, engPath)
			logger.Warn("translate: language check failed with full progress, clearing cache: %s", name)
		}
		subtitle.MarkError(notLangErr)
		_ = subtitle.TransitionTo(valueobject.StatusError)
		_ = s.subtitleRepo.Save(ctx, subtitle)
		return notLangErr
	}

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

	logger.Info("translate done: %s — %d blocks [lang=%s]", name, len(translated), targetLang)
	s.publish(event.NewTranslationCompleted(engPath))
	return nil
}
