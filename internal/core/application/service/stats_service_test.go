package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/application/port"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/valueobject"
	"subsync/internal/testmocks"
)

// helpers

func newStatsService(
	subRepo *testmocks.MockSubtitleRepository,
	keyRepo *testmocks.MockAPIKeyRepository,
	wdRepo *testmocks.MockWatchDirRepository,
	queue *testmocks.MockTaskQueue,
) *StatsService {
	return NewStatsService(subRepo, keyRepo, wdRepo, queue)
}

func makeSubtitle(t *testing.T, status valueobject.SubtitleStatus, embedded bool) *entity.Subtitle {
	t.Helper()
	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	require.NoError(t, err)
	s, err := entity.RestoreSubtitle(
		uuid.New(), mi, "/media/movie.eng.srt",
		status, "", embedded,
		time.Now(), time.Now(),
	)
	require.NoError(t, err)
	return s
}

func makeAPIKey(t *testing.T) *entity.APIKey {
	t.Helper()
	k, err := entity.NewAPIKey("gemini", "key-abc-123")
	require.NoError(t, err)
	return k
}

// --- Stats ---

func TestStatsService_GetStats_DelegatesToRepo(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	want := port.SubtitleStats{Total: 5, Done: 3, Queued: 2}
	subRepo.On("Statistics", mock.Anything).Return(want, nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	got, err := svc.GetStats(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestStatsService_ListRecords_DelegatesToRepo(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	want := []*entity.Subtitle{makeSubtitle(t, valueobject.StatusQueued, false)}
	subRepo.On("FindAll", mock.Anything).Return(want, nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	got, err := svc.ListRecords(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestStatsService_FindByPath_DelegatesToRepo(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	want := makeSubtitle(t, valueobject.StatusDone, false)
	subRepo.On("FindByPath", mock.Anything, "/media/movie.eng.srt").Return(want, nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	got, err := svc.FindByPath(context.Background(), "/media/movie.eng.srt")

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// --- ReTranslate ---

func TestStatsService_ReTranslate_TransitionsToQueued_ThenEnqueues(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer queue.AssertExpectations(t)

	subtitle := makeSubtitle(t, valueobject.StatusDone, false)
	engPath := subtitle.EngPath()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	queue.On("Enqueue", mock.Anything, "translate_srt", port.TranslateTask{EngPath: engPath}).Return(nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, queue)
	err := svc.ReTranslate(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusQueued, subtitle.Status())
}

func TestStatsService_ReTranslate_InvalidTransition_ReturnsError(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	// StatusQueued → StatusQueued is invalid
	subtitle := makeSubtitle(t, valueobject.StatusQueued, false)
	engPath := subtitle.EngPath()
	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ReTranslate(context.Background(), engPath)

	assert.Error(t, err)
}

// --- ReEmbed ---

func TestStatsService_ReEmbed_FromEmbedded_TransitionsToDone(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	subtitle := makeSubtitle(t, valueobject.StatusEmbedded, true)
	engPath := subtitle.EngPath()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ReEmbed(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())
	assert.False(t, subtitle.Embedded())
}

func TestStatsService_ReEmbed_FromEmbedFailed_TransitionsToDone(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	subtitle := makeSubtitle(t, valueobject.StatusEmbedFailed, false)
	engPath := subtitle.EngPath()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ReEmbed(context.Background(), engPath)

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())
}

func TestStatsService_ReEmbed_FromQueued_DoesNotTransition(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	// Queued status: ReEmbed should save without transitioning (not Embedded/EmbedFailed)
	subtitle := makeSubtitle(t, valueobject.StatusQueued, false)
	engPath := subtitle.EngPath()

	subRepo.On("FindByPath", mock.Anything, engPath).Return(subtitle, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ReEmbed(context.Background(), engPath)

	// No error — just saves without transitioning
	require.NoError(t, err)
	// Status unchanged
	assert.Equal(t, valueobject.StatusQueued, subtitle.Status())
}

// --- APIKey management ---

func TestStatsService_AddApiKey_ValidInput_SavesCalled(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	keyRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.APIKey")).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.AddApiKey(context.Background(), "gemini", "my-key", "gemini-3.1-flash-lite")

	require.NoError(t, err)
}

func TestStatsService_AddApiKey_EmptyService_ReturnsError(t *testing.T) {
	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.AddApiKey(context.Background(), "", "my-key", "")
	assert.Error(t, err)
}

func TestStatsService_AddApiKey_EmptyKeyValue_ReturnsError(t *testing.T) {
	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.AddApiKey(context.Background(), "gemini", "", "")
	assert.Error(t, err)
}

func TestStatsService_DisableApiKey_DeactivatesAndSaves(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	key := makeAPIKey(t)
	require.True(t, key.IsActive())

	keyRepo.On("FindByID", mock.Anything, 1).Return(key, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.DisableApiKey(context.Background(), 1)

	require.NoError(t, err)
	assert.False(t, key.IsActive())
}

func TestStatsService_ResetQuotaApiKey_ResetsAndSaves(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	key := makeAPIKey(t)
	key.MarkAsQuotaExceeded(time.Now().Add(24*time.Hour), "")
	require.True(t, key.IsQuotaExceeded())

	keyRepo.On("FindByID", mock.Anything, 5).Return(key, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ResetQuotaApiKey(context.Background(), 5)

	require.NoError(t, err)
	assert.False(t, key.IsQuotaExceeded())
}

func TestStatsService_ActivateAPIKey_ActivatesAndSaves(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	key := makeAPIKey(t)
	key.Deactivate()
	require.False(t, key.IsActive())

	keyRepo.On("FindByID", mock.Anything, 3).Return(key, nil)
	keyRepo.On("Save", mock.Anything, key).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.ActivateAPIKey(context.Background(), 3)

	require.NoError(t, err)
	assert.True(t, key.IsActive())
}

func TestStatsService_DeleteAPIKey_DelegatesToRepo(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	keyRepo.On("Delete", mock.Anything, 9).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.DeleteAPIKey(context.Background(), 9)

	require.NoError(t, err)
}

func TestStatsService_ListAPIKeys_DelegatesToRepo(t *testing.T) {
	keyRepo := &testmocks.MockAPIKeyRepository{}
	defer keyRepo.AssertExpectations(t)

	want := []*entity.APIKey{makeAPIKey(t)}
	keyRepo.On("FindAll", mock.Anything).Return(want, nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, keyRepo, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	got, err := svc.ListAPIKeys(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

// --- WatchDir management ---

func TestStatsService_AddWatchDir_ValidPath_SavesCalled(t *testing.T) {
	wdRepo := &testmocks.MockWatchDirRepository{}
	defer wdRepo.AssertExpectations(t)

	wdRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.WatchDir")).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, wdRepo, &testmocks.MockTaskQueue{})
	err := svc.AddWatchDir(context.Background(), t.TempDir())

	require.NoError(t, err)
}

func TestStatsService_AddWatchDir_InvalidPath_ReturnsError(t *testing.T) {
	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	err := svc.AddWatchDir(context.Background(), "relative/path")
	assert.Error(t, err)
}

func TestStatsService_DeleteWatchDir_DelegatesToRepo(t *testing.T) {
	wdRepo := &testmocks.MockWatchDirRepository{}
	defer wdRepo.AssertExpectations(t)

	wdRepo.On("Delete", mock.Anything, 7).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, wdRepo, &testmocks.MockTaskQueue{})
	err := svc.DeleteWatchDir(context.Background(), 7)

	require.NoError(t, err)
}

func TestStatsService_ToggleWatchDir_TogglesAndSaves(t *testing.T) {
	wdRepo := &testmocks.MockWatchDirRepository{}
	defer wdRepo.AssertExpectations(t)

	wd := entity.RestoreWatchDir(2, "/media/tv", true, time.Now())
	require.True(t, wd.IsEnabled())

	wdRepo.On("FindByID", mock.Anything, 2).Return(wd, nil)
	wdRepo.On("Save", mock.Anything, wd).Return(nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, wdRepo, &testmocks.MockTaskQueue{})
	err := svc.ToggleWatchDir(context.Background(), 2)

	require.NoError(t, err)
	assert.False(t, wd.IsEnabled())
}

func TestStatsService_ListWatchDirs_DelegatesToRepo(t *testing.T) {
	wdRepo := &testmocks.MockWatchDirRepository{}
	defer wdRepo.AssertExpectations(t)

	want := []*entity.WatchDir{entity.RestoreWatchDir(1, "/media/tv", true, time.Now())}
	wdRepo.On("FindAll", mock.Anything).Return(want, nil)

	svc := newStatsService(&testmocks.MockSubtitleRepository{}, &testmocks.MockAPIKeyRepository{}, wdRepo, &testmocks.MockTaskQueue{})
	got, err := svc.ListWatchDirs(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestStatsService_ListRecordsByStatus_DelegatesToRepo(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	want := []*entity.Subtitle{makeSubtitle(t, valueobject.StatusDone, false)}
	subRepo.On("FindByStatus", mock.Anything, valueobject.StatusDone).Return(want, nil)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	got, err := svc.ListRecordsByStatus(context.Background(), valueobject.StatusDone)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestStatsService_GetStats_RepoError_Propagated(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	defer subRepo.AssertExpectations(t)

	repoErr := errors.New("db connection lost")
	subRepo.On("Statistics", mock.Anything).Return(port.SubtitleStats{}, repoErr)

	svc := newStatsService(subRepo, &testmocks.MockAPIKeyRepository{}, &testmocks.MockWatchDirRepository{}, &testmocks.MockTaskQueue{})
	_, err := svc.GetStats(context.Background())

	assert.ErrorIs(t, err, repoErr)
}
