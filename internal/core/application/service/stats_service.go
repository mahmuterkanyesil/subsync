package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
)

type modelSpec struct{ rpm, tpm, rpd int }

// knownModelSpecs maps Gemini model IDs to their free-tier rate limits.
var knownModelSpecs = map[string]modelSpec{
	"gemini-3.1-pro": {rpm: 2, tpm: 32_000, rpd: 50},
	"gemini-3.1-flash-lite": {rpm: 15, tpm: 250_000, rpd: 500},
	"gemini-3.1-flash":      {rpm: 15, tpm: 1_000_000, rpd: 1500},
	"gemini-2.5-flash-lite": {rpm: 10, tpm: 250_000, rpd: 50},
	"gemini-2.5-flash":        {rpm: 5, tpm: 250_000, rpd: 25},
	"gemini-3-flash-preview":  {rpm: 5, tpm: 250_000, rpd: 25},
}

func applyModel(key *entity.APIKey, model string) {
	if model == "" {
		return
	}
	key.SetModel(model)
	if spec, ok := knownModelSpecs[model]; ok {
		key.UpdateLimits(spec.rpm, spec.tpm, spec.rpd)
	}
}

type StatsService struct {
	subtitleRepo port.SubtitleRepository
	apiKeyRepo   port.APIKeyRepository
	watchDirRepo port.WatchDirRepository
	taskQueue    port.TaskQueue
	settingsRepo port.AppSettingsRepository
}

func NewStatsService(
	subtitleRepo port.SubtitleRepository,
	apiKeyRepo port.APIKeyRepository,
	watchDirRepo port.WatchDirRepository,
	taskQueue port.TaskQueue,
	settingsRepo port.AppSettingsRepository,
) *StatsService {
	return &StatsService{
		subtitleRepo: subtitleRepo,
		apiKeyRepo:   apiKeyRepo,
		watchDirRepo: watchDirRepo,
		taskQueue:    taskQueue,
		settingsRepo: settingsRepo,
	}
}

func (s *StatsService) GetStats(ctx context.Context) (*port.SubtitleStats, error) {
	return s.subtitleRepo.Statistics(ctx)
}

func (s *StatsService) ListRecords(ctx context.Context) ([]*entity.Subtitle, error) {
	return s.subtitleRepo.FindAll(ctx)
}

func (s *StatsService) FindByPath(ctx context.Context, engPath string) (*entity.Subtitle, error) {
	return s.subtitleRepo.FindByPath(ctx, engPath)
}

func (s *StatsService) DeleteSubtitle(ctx context.Context, engPath string) error {
	return s.subtitleRepo.Delete(ctx, engPath)
}

func (s *StatsService) GetTargetLanguage(ctx context.Context) string {
	if s.settingsRepo != nil {
		if v, err := s.settingsRepo.GetSetting(ctx, "target_language"); err == nil && v != "" {
			return v
		}
	}
	return "tr"
}

func (s *StatsService) SetTargetLanguage(ctx context.Context, code string) error {
	if _, ok := valueobject.LookupLanguage(code); !ok {
		return fmt.Errorf("unsupported language code: %s", code)
	}
	if s.settingsRepo == nil {
		return fmt.Errorf("settings repository not available")
	}
	return s.settingsRepo.SetSetting(ctx, "target_language", code)
}

const maxRetries = 3

func (s *StatsService) ReTranslate(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}
	if err := subtitle.TransitionTo(valueobject.StatusQueued); err != nil {
		return err
	}
	subtitle.IncrementRetry()
	if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
		return err
	}
	return s.taskQueue.Enqueue(ctx, "translate_srt", port.TranslateTask{
		EngPath:        engPath,
		TargetLanguage: s.GetTargetLanguage(ctx),
	})
}

func (s *StatsService) ReEmbed(ctx context.Context, engPath string) error {
	subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
	if err != nil {
		return err
	}
	// embedded → done veya embed_failed → done geçişi yaparak embedder'ın tekrar almasını sağla
	status := subtitle.Status()
	if status == valueobject.StatusEmbedded || status == valueobject.StatusEmbedFailed {
		if err := subtitle.TransitionTo(valueobject.StatusDone); err != nil {
			return err
		}
		subtitle.MarkUnembedded()
	}
	return s.subtitleRepo.Save(ctx, subtitle)
}

func (s *StatsService) AddApiKey(ctx context.Context, service, keyValue, model string) error {
	key, err := entity.NewAPIKey(service, keyValue)
	if err != nil {
		return err
	}
	applyModel(key, model)
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) UpdateApiKeyModel(ctx context.Context, id int, model string) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	applyModel(key, model)
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) DisableApiKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.Deactivate()
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) ResetQuotaApiKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.ResetQuota()
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) ListAPIKeys(ctx context.Context) ([]*entity.APIKey, error) {
	_ = s.apiKeyRepo.ResetExpiredQuotas(ctx)
	return s.apiKeyRepo.FindAll(ctx)
}

func (s *StatsService) ListAPIKeysWithUsage(ctx context.Context) ([]port.APIKeyWithUsage, error) {
	_ = s.apiKeyRepo.ResetExpiredQuotas(ctx)
	keys, err := s.apiKeyRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]port.APIKeyWithUsage, len(keys))
	for i, k := range keys {
		usage, _ := s.apiKeyRepo.FindAllModelUsage(ctx, k.ID())
		for j := range usage {
			if spec, ok := knownModelSpecs[usage[j].Model]; ok {
				usage[j].RPDLimit = spec.rpd
			}
		}
		result[i] = port.APIKeyWithUsage{Key: k, ModelUsage: usage}
	}
	return result, nil
}

func (s *StatsService) RefreshKeyStatuses(ctx context.Context) error {
	return s.apiKeyRepo.ResetExpiredQuotas(ctx)
}

func (s *StatsService) DeleteAPIKey(ctx context.Context, id int) error {
	return s.apiKeyRepo.Delete(ctx, id)
}

func (s *StatsService) ActivateAPIKey(ctx context.Context, id int) error {
	key, err := s.apiKeyRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	key.Activate()
	return s.apiKeyRepo.Save(ctx, key)
}

func (s *StatsService) ListRecordsByStatus(ctx context.Context, status valueobject.SubtitleStatus) ([]*entity.Subtitle, error) {
	return s.subtitleRepo.FindByStatus(ctx, status)
}

func (s *StatsService) SearchRecords(ctx context.Context, f port.SubtitleFilter) (*port.SubtitlePage, error) {
	return s.subtitleRepo.FindWithFilters(ctx, f)
}

func (s *StatsService) BulkReTranslate(ctx context.Context, engPaths []string) error {
	targetLang := s.GetTargetLanguage(ctx)
	for _, engPath := range engPaths {
		subtitle, err := s.subtitleRepo.FindByPath(ctx, engPath)
		if err != nil {
			continue
		}
		if !subtitle.CanRetry(maxRetries) {
			continue
		}
		if err := subtitle.TransitionTo(valueobject.StatusQueued); err != nil {
			continue
		}
		subtitle.IncrementRetry()
		if err := s.subtitleRepo.Save(ctx, subtitle); err != nil {
			continue
		}
		_ = s.taskQueue.Enqueue(ctx, "translate_srt", port.TranslateTask{
			EngPath:        engPath,
			TargetLanguage: targetLang,
		})
	}
	return nil
}

func (s *StatsService) BulkDelete(ctx context.Context, engPaths []string) error {
	return s.subtitleRepo.DeleteMany(ctx, engPaths)
}

const modelPriorityKey = "model_priority"

var defaultModelPriority = []string{
	"gemini-3.1-flash",
	"gemini-3.1-flash-lite",
	"gemini-2.5-flash",
	"gemini-2.5-flash-lite",
	"gemini-3-flash-preview",
	"gemini-3.1-pro",
}

func (s *StatsService) GetModelPriority(ctx context.Context) []string {
	if s.settingsRepo == nil {
		return defaultModelPriority
	}
	v, err := s.settingsRepo.GetSetting(ctx, modelPriorityKey)
	if err != nil || v == "" {
		return defaultModelPriority
	}
	var models []string
	if err := json.Unmarshal([]byte(v), &models); err != nil {
		return defaultModelPriority
	}
	return models
}

func (s *StatsService) SetModelPriority(ctx context.Context, models []string) error {
	if s.settingsRepo == nil {
		return fmt.Errorf("settings repository not available")
	}
	data, err := json.Marshal(models)
	if err != nil {
		return err
	}
	return s.settingsRepo.SetSetting(ctx, modelPriorityKey, string(data))
}

func (s *StatsService) GetTranslationPreview(ctx context.Context, engPath string) (string, error) {
	targetLang := s.GetTargetLanguage(ctx)
	trPath := findTrSrtPath(engPath, targetLang)
	if trPath == "" {
		return "", fmt.Errorf("translation file not found next to %s", filepath.Base(engPath))
	}

	data, err := os.ReadFile(trPath)
	if err != nil {
		return "", fmt.Errorf("translation file not found: %w", err)
	}
	return string(data), nil
}

// findTrSrtPath locates the translated .srt file adjacent to engPath.
// It tries the canonical path first, then falls back to a glob search so it
// works regardless of language code mismatches or legacy naming formats.
func findTrSrtPath(engPath, targetLang string) string {
	dir := filepath.Dir(engPath)

	baseName := filepath.Base(engPath)
	base := strings.TrimSuffix(baseName, ".eng.srt")
	base = strings.TrimSuffix(base, ".srt")

	// 1. canonical: base.tr.srt
	if p := filepath.Join(dir, base+"."+targetLang+".srt"); fileExists(p) {
		return p
	}
	// 2. legacy: base.eng.tr.srt
	legacyBase := strings.TrimSuffix(baseName, filepath.Ext(baseName))
	if p := filepath.Join(dir, legacyBase+"."+targetLang+".srt"); fileExists(p) {
		return p
	}
	// 3. glob fallback: any base.*.srt that isn't the source file
	matches, _ := filepath.Glob(filepath.Join(dir, base+".*.srt"))
	for _, m := range matches {
		if m != engPath {
			return m
		}
	}
	return ""
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (s *StatsService) ListWatchDirs(ctx context.Context) ([]*entity.WatchDir, error) {
	return s.watchDirRepo.FindAll(ctx)
}

func (s *StatsService) AddWatchDir(ctx context.Context, path string) error {
	wd, err := entity.NewWatchDir(path)
	if err != nil {
		return err
	}
	return s.watchDirRepo.Save(ctx, wd)
}

func (s *StatsService) DeleteWatchDir(ctx context.Context, id int) error {
	return s.watchDirRepo.Delete(ctx, id)
}

func (s *StatsService) ToggleWatchDir(ctx context.Context, id int) error {
	wd, err := s.watchDirRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	wd.Toggle()
	return s.watchDirRepo.Save(ctx, wd)
}
