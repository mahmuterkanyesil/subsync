package service

import (
	"context"
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

// makeSubtitleForEmbed creates a Subtitle pointing to a real .eng.srt file in tmpDir.
func makeSubtitleForEmbed(t *testing.T, tmpDir, baseName string, status valueobject.SubtitleStatus) *entity.Subtitle {
	t.Helper()
	engPath := filepath.Join(tmpDir, baseName+".eng.srt")
	// Write a small valid .eng.srt
	err := os.WriteFile(engPath, []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n\n"), 0644)
	require.NoError(t, err)

	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	require.NoError(t, err)
	s, err := entity.RestoreSubtitle(uuid.New(), mi, engPath, status, "", false, time.Now(), time.Now())
	require.NoError(t, err)
	return s
}

// ---- findVideoPath whitebox tests ----

func TestFindVideoPath(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(dir string) string // returns engPath
		wantExt  string
		wantErr  error
	}{
		{
			name: "mkv exists",
			setup: func(dir string) string {
				os.WriteFile(filepath.Join(dir, "movie.mkv"), []byte{}, 0644)
				return filepath.Join(dir, "movie.eng.srt")
			},
			wantExt: ".mkv",
		},
		{
			name: "mp4 exists",
			setup: func(dir string) string {
				os.WriteFile(filepath.Join(dir, "movie.mp4"), []byte{}, 0644)
				return filepath.Join(dir, "movie.eng.srt")
			},
			wantExt: ".mp4",
		},
		{
			name: "neither exists",
			setup: func(dir string) string {
				return filepath.Join(dir, "movie.eng.srt")
			},
			wantErr: port.ErrVideoNotFound,
		},
		{
			name: "both mkv and mp4 exist — mkv preferred",
			setup: func(dir string) string {
				os.WriteFile(filepath.Join(dir, "movie.mkv"), []byte{}, 0644)
				os.WriteFile(filepath.Join(dir, "movie.mp4"), []byte{}, 0644)
				return filepath.Join(dir, "movie.eng.srt")
			},
			wantExt: ".mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			engPath := tt.setup(dir)

			videoPath, err := findVideoPath(engPath)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantExt, filepath.Ext(videoPath))
		})
	}
}

// ---- EmbedPending service tests ----

func newEmbedSvc(
	subRepo *testmocks.MockSubtitleRepository,
	vp *testmocks.MockVideoProcessor,
	events port.EventPublisher,
) *EmbeddingService {
	return NewEmbeddingService(subRepo, vp, events)
}

func TestEmbeddingService_EmbedPending_EmptyList_NoOp(t *testing.T) {
	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer subRepo.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{}, nil)
	vp.AssertNotCalled(t, "HasTurkishSubtitle")
	vp.AssertNotCalled(t, "EmbedSubtitle")

	svc := newEmbedSvc(subRepo, vp, nil)
	err := svc.EmbedPending(context.Background())
	require.NoError(t, err)
}

func TestEmbeddingService_EmbedPending_HappyPath_MkvFound(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "movie", valueobject.StatusDone)
	engPath := subtitle.EngPath()

	// Create the .mkv stub
	mkvPath := filepath.Join(dir, "movie.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(false, nil)
	vp.On("EmbedSubtitle", mock.Anything, mkvPath, mock.AnythingOfType("string")).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "embedding.completed"
	})).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err := svc.EmbedPending(context.Background())

	require.NoError(t, err)
	assert.True(t, subtitle.Embedded())
	assert.Equal(t, engPath, subtitle.EngPath())
}

func TestEmbeddingService_EmbedPending_VideoNotFound_TransitionsEmbedFailed(t *testing.T) {
	dir := t.TempDir()
	// No video file created — findVideoPath will return ErrVideoNotFound
	subtitle := makeSubtitleForEmbed(t, dir, "movie", valueobject.StatusDone)

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer subRepo.AssertExpectations(t)
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "embedding.failed"
	})).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err := svc.EmbedPending(context.Background())

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusEmbedFailed, subtitle.Status())
	vp.AssertNotCalled(t, "HasTurkishSubtitle")
}

func TestEmbeddingService_EmbedPending_AlreadyHasTurkishSub_MarksEmbedded(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "episode", valueobject.StatusDone)

	mkvPath := filepath.Join(dir, "episode.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	// Turkish subtitle already embedded → no EmbedSubtitle call
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(true, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "embedding.completed"
	})).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err := svc.EmbedPending(context.Background())

	require.NoError(t, err)
	assert.True(t, subtitle.Embedded())
	vp.AssertNotCalled(t, "EmbedSubtitle")
}

func TestEmbeddingService_EmbedPending_EngSrtTooLarge_TransitionsEmbedFailed(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "large", valueobject.StatusDone)
	engPath := subtitle.EngPath()

	mkvPath := filepath.Join(dir, "large.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	// Expand .eng.srt beyond 2MB
	f, err := os.OpenFile(engPath, os.O_RDWR, 0644)
	require.NoError(t, err)
	require.NoError(t, f.Truncate(3*1024*1024))
	f.Close()

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(false, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)
	events.On("Publish", mock.MatchedBy(func(e event.DomainEvent) bool {
		return e.EventName() == "embedding.failed"
	})).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err = svc.EmbedPending(context.Background())

	require.NoError(t, err)
	assert.Equal(t, valueobject.StatusEmbedFailed, subtitle.Status())
	vp.AssertNotCalled(t, "EmbedSubtitle")
}

func TestEmbeddingService_EmbedPending_FFmpegFailed_StaysInDone(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "ffmpeg_fail", valueobject.StatusDone)

	mkvPath := filepath.Join(dir, "ffmpeg_fail.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(false, nil)
	vp.On("EmbedSubtitle", mock.Anything, mkvPath, mock.AnythingOfType("string")).Return(port.ErrFFmpegFailed)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newEmbedSvc(subRepo, vp, nil)
	err := svc.EmbedPending(context.Background())

	require.NoError(t, err)
	// Transient error — subtitle stays at StatusDone
	assert.Equal(t, valueobject.StatusDone, subtitle.Status())
	assert.False(t, subtitle.Embedded())
}

func TestEmbeddingService_EmbedPending_TrSrtNotFound_TransitionsQueued(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "notrsrt", valueobject.StatusDone)

	mkvPath := filepath.Join(dir, "notrsrt.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(false, nil)
	vp.On("EmbedSubtitle", mock.Anything, mkvPath, mock.AnythingOfType("string")).Return(port.ErrTrSrtNotFound)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	svc := newEmbedSvc(subRepo, vp, nil)
	err := svc.EmbedPending(context.Background())

	require.NoError(t, err)
	// ErrTrSrtNotFound → transition to StatusQueued so translation re-runs
	assert.Equal(t, valueobject.StatusQueued, subtitle.Status())
}

func TestEmbeddingService_EmbedPending_DuplicateInProgress_Skipped(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "inprogress", valueobject.StatusDone)

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer subRepo.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	// videoProcessor must NOT be called
	vp.AssertNotCalled(t, "HasTurkishSubtitle")
	vp.AssertNotCalled(t, "EmbedSubtitle")

	svc := newEmbedSvc(subRepo, vp, nil)
	// Pre-populate inProgress map (whitebox: same package)
	svc.inProgress[subtitle.EngPath()] = struct{}{}

	err := svc.EmbedPending(context.Background())
	require.NoError(t, err)
}

func TestEmbeddingService_EmbedPending_PublishesEmbeddingCompleted(t *testing.T) {
	dir := t.TempDir()
	subtitle := makeSubtitleForEmbed(t, dir, "pub_ok", valueobject.StatusDone)

	mkvPath := filepath.Join(dir, "pub_ok.mkv")
	require.NoError(t, os.WriteFile(mkvPath, []byte{}, 0644))

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	vp.On("HasTurkishSubtitle", mock.Anything, mkvPath).Return(false, nil)
	vp.On("EmbedSubtitle", mock.Anything, mkvPath, mock.AnythingOfType("string")).Return(nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	var published event.DomainEvent
	events.On("Publish", mock.AnythingOfType("event.EmbeddingCompleted")).
		Run(func(args mock.Arguments) {
			published = args.Get(0).(event.DomainEvent)
		}).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err := svc.EmbedPending(context.Background())
	require.NoError(t, err)

	require.NotNil(t, published)
	assert.Equal(t, "embedding.completed", published.EventName())
}

func TestEmbeddingService_EmbedPending_PublishesEmbeddingFailed(t *testing.T) {
	dir := t.TempDir()
	// No video file → ErrVideoNotFound → EmbeddingFailed event
	subtitle := makeSubtitleForEmbed(t, dir, "pub_fail", valueobject.StatusDone)

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	events := &testmocks.MockEventPublisher{}
	defer events.AssertExpectations(t)

	subRepo.On("FindPendingEmbed", mock.Anything).Return([]*entity.Subtitle{subtitle}, nil)
	subRepo.On("Save", mock.Anything, subtitle).Return(nil)

	var published event.DomainEvent
	events.On("Publish", mock.AnythingOfType("event.EmbeddingFailed")).
		Run(func(args mock.Arguments) {
			published = args.Get(0).(event.DomainEvent)
		}).Return()

	svc := newEmbedSvc(subRepo, vp, events)
	err := svc.EmbedPending(context.Background())
	require.NoError(t, err)

	require.NotNil(t, published)
	assert.Equal(t, "embedding.failed", published.EventName())
}
