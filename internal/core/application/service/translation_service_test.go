package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/event"
	"subsync/internal/core/domain/valueobject"
	"subsync/internal/testmocks"
)

// makeEngSRT writes an SRT file with the given content and returns its path.
func makeEngSRT(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "movie.eng.srt")
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// srtContent builds a minimal 2-block SRT file content.
func srtContent() string {
	return "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n" +
		"2\n00:00:03,000 --> 00:00:04,000\nWorld\n\n"
}

// turkishBlocks returns 2 SRT blocks with Turkish text.
func turkishBlocks() []port.SRTBlock {
	return []port.SRTBlock{
		{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Bugün güzel bir gün"},
		{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "Şimdi buradayım"},
	}
}

func makeSubtitleWithPath(t *testing.T, engPath string, status valueobject.SubtitleStatus) *entity.Subtitle {
	t.Helper()
	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	require.NoError(t, err)
	s, err := entity.RestoreSubtitle(uuid.New(), mi, engPath, status, "", false, time.Now(), time.Now())
	require.NoError(t, err)
	return s
}

func newTranslateSvc(
	subRepo *testmocks.MockSubtitleRepository,
	keyRepo *testmocks.MockAPIKeyRepository,
	translator *testmocks.MockTranslationProvider,
	progress port.ProgressStore,
	events port.EventPublisher,
) *TranslationService {
	// batchSize=100 covers all blocks in one batch (avoids 10s inter-batch sleep)
	return NewTranslationService(subRepo, keyRepo, translator, progress, events, 100)
}

func makeKey(t *testing.T) *entity.APIKey {
	t.Helper()
	k, err := entity.NewAPIKey("gemini", "test-key-abc")
	require.NoError(t, err)
	return k
}

// ---- Tests ----

func TestTranslationService_Translate_AlreadyDone_NoOp(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusDone)

	subRepo := &testmocks.MockSubtitleRepository{}
	translator := &testmocks.MockTranslationProvider{}
	defer subRepo.AssertExpectations(t)

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	translator.AssertNotCalled(t, "TranslateBatch")

	svc := newTranslateSvc(subRepo, &testmocks.MockAPIKeyRepository{}, translator, &testmocks.MockProgressStore{}, nil)
	err := svc.Translate(context.Background(), engPath)
	require.NoError(t, err)
}

func TestTranslationService_Translate_HappyPath_SingleBatch(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	events := &testmocks.MockEventPublisher{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)
	defer events.AssertExpectations(t)

	key := makeKey(t)
	translated := turkishBlocks()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue()).
		Return(translated, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	progressStore.On("Clear", mock.Anything, engPath).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "translation.completed"
	})).Return()

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, events)
	err := svc.Translate(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())

	// .tr.srt must be written — service uses TrimSuffix(engPath, ".srt") + ".tr.srt"
	// which gives "movie.eng" + ".tr.srt" = "movie.eng.tr.srt"
	trPath := engPath[:len(engPath)-len(".srt")] + ".tr.srt"
	_, statErr := os.Stat(trPath)
	assert.NoError(t, statErr, ".tr.srt file should be created")
}

func TestTranslationService_Translate_ResumeFromProgress(t *testing.T) {
	// 2-block SRT; progress has first block already → only 1 block sent to TranslateBatch
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	key := makeKey(t)
	alreadyTranslated := turkishBlocks()[:1] // first block already done
	remainingTranslated := turkishBlocks()[1:] // second block returned

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	// Progress has the first block
	progressStore.On("Load", mock.Anything, engPath).Return(alreadyTranslated, true, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	// Only the second block (remaining[0]) should be sent
	translator.On("TranslateBatch", mock.Anything, mock.MatchedBy(func(blocks []port.SRTBlock) bool {
		return len(blocks) == 1
	}), key.KeyValue()).Return(remainingTranslated, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	progressStore.On("Clear", mock.Anything, engPath).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())
}

func TestTranslationService_Translate_ValidationFails_TransitionsToError(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	key := makeKey(t)
	// English text → validation will fail
	englishBlocks := []port.SRTBlock{
		{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "He went to the store"},
		{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "She and her friend waited"},
	}

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue()).
		Return(englishBlocks, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Equal(t, valueobject.StatusError, subtitle.Status())
}

func TestTranslationService_Translate_QuotaExhaustedRPD_TransitionsToQuotaExhausted(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	// 3 different keys, all fail with quota_exhausted_rpd
	key1, _ := entity.NewAPIKey("gemini", "key-1")
	key2, _ := entity.NewAPIKey("gemini", "key-2")
	key3, _ := entity.NewAPIKey("gemini", "key-3")
	quotaErr := errors.New("quota_exhausted_rpd")

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	// FindNextAvailable called 3 times (maxRetry=3), returns a different key each time
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key1, nil).Once()
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key2, nil).Once()
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key3, nil).Once()

	// Each key's TranslateBatch returns quota error
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), "key-1").Return(nil, quotaErr)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), "key-2").Return(nil, quotaErr)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), "key-3").Return(nil, quotaErr)

	// Each exhausted key gets saved
	keyRepo.On("Save", mock.Anything, key1).Return(nil)
	keyRepo.On("Save", mock.Anything, key2).Return(nil)
	keyRepo.On("Save", mock.Anything, key3).Return(nil)

	// After max retries, .tr.srt cleanup and final subtitle save
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "quota exhausted")
	assert.Equal(t, valueobject.StatusQuotaExhausted, subtitle.Status())
}

func TestTranslationService_Translate_QuotaExhaustedRPM_CtxCancel_SavesProgress(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	key := makeKey(t)
	rpmErr := errors.New("quota_exhausted_rpm")

	// Use a context that cancels very quickly to avoid the 60s sleep
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue()).
		Return(nil, rpmErr)
	// Progress saved before the 60s wait
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(ctx, engPath)

	// Should return context error (deadline exceeded)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
	// Progress must have been saved
	progressStore.AssertCalled(t, "Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock"))
}

func TestTranslationService_Translate_NoAPIKey_ReturnsError(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(nil, errors.New("no keys"))
	// Progress saved before returning error
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	translator.AssertNotCalled(t, "TranslateBatch")

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no available api key")
	progressStore.AssertCalled(t, "Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock"))
}

func TestTranslationService_Translate_PublishesEventOnSuccess(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	events := &testmocks.MockEventPublisher{}
	defer events.AssertExpectations(t)

	key := makeKey(t)
	translated := turkishBlocks()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue()).
		Return(translated, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	progressStore.On("Clear", mock.Anything, engPath).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	var published event.DomainEvent
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "translation.completed"
	})).Run(func(args mock.Arguments) {
		published = args.Get(0).(event.DomainEvent)
	}).Return()

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, events)
	err := svc.Translate(context.Background(), engPath)

	require.NoError(t, err)
	require.NotNil(t, published)
	assert.Equal(t, "translation.completed", published.EventName())

	tc, ok := published.(event.TranslationCompleted)
	require.True(t, ok)
	assert.Equal(t, engPath, tc.EngPath)
}

func TestTranslationService_Translate_DefaultOtherError_SavesProgressAndReturns(t *testing.T) {
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer translator.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	key := makeKey(t)
	networkErr := fmt.Errorf("network timeout")

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue()).
		Return(nil, networkErr)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.ErrorIs(t, err, networkErr)
	progressStore.AssertCalled(t, "Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock"))
}
