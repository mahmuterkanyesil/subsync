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

// turkishBlocks returns 2 SRT blocks with Turkish text containing Turkish-unique chars.
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
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	translator.AssertNotCalled(t, "TranslateBatch")

	svc := newTranslateSvc(subRepo, keyRepo, translator, &testmocks.MockProgressStore{}, nil)
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

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
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

	// .tr.srt must be written
	trPath := engPath[:len(engPath)-len(".srt")] + ".tr.srt"
	_, statErr := os.Stat(trPath)
	assert.NoError(t, statErr, ".tr.srt file should be created")
}

func TestTranslationService_Translate_ResumeFromProgress(t *testing.T) {
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
	alreadyTranslated := turkishBlocks()[:1]
	remainingTranslated := turkishBlocks()[1:]

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(alreadyTranslated, true, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.MatchedBy(func(blocks []port.SRTBlock) bool {
		return len(blocks) == 1
	}), key.KeyValue(), mock.Anything).Return(remainingTranslated, nil)
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
	englishBlocks := []port.SRTBlock{
		{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "He went to the store"},
		{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "She and her friend waited"},
	}

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
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

	quotaErr := errors.New("quota_exhausted_rpd")

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	// maxRetry = 10: set up 10 keys all returning quota_exhausted_rpd
	for i := 1; i <= 10; i++ {
		k, _ := entity.NewAPIKey("gemini", fmt.Sprintf("key-%d", i))
		keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(k, nil).Once()
		translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), fmt.Sprintf("key-%d", i), mock.Anything).Return(nil, quotaErr)
		keyRepo.On("Save", mock.Anything, k).Return(nil)
	}

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

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
		Return(nil, rpmErr)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(ctx, engPath)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
	progressStore.AssertCalled(t, "Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock"))
}

func TestTranslationService_Translate_NoAPIKey_ReturnsError(t *testing.T) {
	// When FindNextAvailable fails AND FindEarliestQuotaReset returns nil (no keys configured),
	// the service must transition to StatusQuotaExhausted immediately.
	engPath := makeEngSRT(t, srtContent())
	subtitle := makeSubtitleWithPath(t, engPath, valueobject.StatusQueued)

	subRepo := &testmocks.MockSubtitleRepository{}
	keyRepo := &testmocks.MockAPIKeyRepository{}
	translator := &testmocks.MockTranslationProvider{}
	progressStore := &testmocks.MockProgressStore{}
	defer subRepo.AssertExpectations(t)
	defer keyRepo.AssertExpectations(t)
	defer progressStore.AssertExpectations(t)

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(nil, errors.New("no keys"))
	keyRepo.On("FindEarliestQuotaReset", mock.Anything, "gemini").Return(nil, nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	translator.AssertNotCalled(t, "TranslateBatch")

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no active api keys")
	assert.Equal(t, valueobject.StatusQuotaExhausted, subtitle.Status())
}

func TestTranslationService_Translate_AllKeysExceeded_WaitsForReset(t *testing.T) {
	// When all keys are temporarily quota-exceeded, the service must wait until the
	// earliest reset time and retry — not fail immediately.
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
	translated := turkishBlocks()
	// Reset time in the past → wait duration resolves immediately.
	pastReset := time.Now().Add(-1 * time.Second)

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)

	// First call: all keys exceeded → ErrNoRows
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(nil, errors.New("sql: no rows in result set")).Once()
	keyRepo.On("FindEarliestQuotaReset", mock.Anything, "gemini").Return(&pastReset, nil).Once()

	// After wait: key is now available → translation succeeds
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil).Once()
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
		Return(translated, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)
	progressStore.On("Clear", mock.Anything, engPath).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())
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
	defer keyRepo.AssertExpectations(t)

	key := makeKey(t)
	translated := turkishBlocks()

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
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

	keyRepo.On("ResetExpiredQuotas", mock.Anything).Return(nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	progressStore.On("Load", mock.Anything, engPath).Return(nil, false, nil)
	keyRepo.On("FindNextAvailable", mock.Anything, "gemini").Return(key, nil)
	translator.On("TranslateBatch", mock.Anything, mock.AnythingOfType("[]valueobject.SRTBlock"), key.KeyValue(), mock.Anything).
		Return(nil, networkErr)
	progressStore.On("Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock")).Return(nil)

	svc := newTranslateSvc(subRepo, keyRepo, translator, progressStore, nil)
	err := svc.Translate(context.Background(), engPath)

	assert.ErrorIs(t, err, networkErr)
	progressStore.AssertCalled(t, "Save", mock.Anything, engPath, mock.AnythingOfType("[]valueobject.SRTBlock"))
}
