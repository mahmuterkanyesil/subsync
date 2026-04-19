package service

import (
	"context"
	"errors"
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
	"subsync/internal/core/domain/valueobject"
	"subsync/internal/testmocks"
)

// makeTempFile writes a zero-byte file at dir/name and returns its full path.
func makeTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte{}, 0644)
	require.NoError(t, err)
	return path
}

// makeSubtitleAt creates a subtitle entity pointing to a given engPath.
func makeSubtitleAt(t *testing.T, engPath string, status valueobject.SubtitleStatus) *entity.Subtitle {
	t.Helper()
	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	require.NoError(t, err)
	s, err := entity.RestoreSubtitle(uuid.New(), mi, engPath, status, "", false, time.Now(), time.Now())
	require.NoError(t, err)
	return s
}

// newScanSvc creates a ScanningService wired with the provided mocks and watch dir list.
// wdRepo accepts the interface type so that a nil literal is treated as a nil interface.
func newScanSvc(
	subRepo *testmocks.MockSubtitleRepository,
	vp *testmocks.MockVideoProcessor,
	queue *testmocks.MockTaskQueue,
	watchDirs []string,
	wdRepo port.WatchDirRepository,
) *ScanningService {
	return NewScanningService(subRepo, vp, queue, watchDirs, wdRepo, nil)
}

// ---- extractSxxExx whitebox tests ----

func TestExtractSxxExx(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantSeason  int
		wantEpisode int
		wantOk      bool
	}{
		{"standard S03E07", "/tv/Show/S03E07.mkv", 3, 7, true},
		{"lowercase s01e01", "/tv/Show/s01e01.mkv", 1, 1, true},
		{"double-digit S12E24", "/tv/Show/S12E24.eng.srt", 12, 24, true},
		{"no pattern movie", "/movies/film.mkv", 0, 0, false},
		{"no pattern plain", "movie.mkv", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			season, episode, ok := extractSxxExx(tt.path)
			assert.Equal(t, tt.wantOk, ok)
			assert.Equal(t, tt.wantSeason, season)
			assert.Equal(t, tt.wantEpisode, episode)
		})
	}
}

// ---- inferMediaInfo whitebox tests ----

func TestInferMediaInfo_Series(t *testing.T) {
	mi := inferMediaInfo("/media/Show.S03E07.mkv")
	assert.Equal(t, valueobject.MediaTypeSeries, mi.MediaType)
	assert.Equal(t, 3, mi.SeasonNumber)
	assert.Equal(t, 7, mi.EpisodeNumber)
}

func TestInferMediaInfo_Movie(t *testing.T) {
	mi := inferMediaInfo("/media/movies/Inception.mkv")
	assert.Equal(t, valueobject.MediaTypeMovie, mi.MediaType)
}

// ---- Scan integration-style tests (real tmpdir + mocks) ----

func TestScanningService_Scan_SkipsNonVideoFiles(t *testing.T) {
	dir := t.TempDir()
	makeTempFile(t, dir, "movie.avi")
	makeTempFile(t, dir, "subtitle.srt")

	subRepo := &testmocks.MockSubtitleRepository{}
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	vp := &testmocks.MockVideoProcessor{}
	// HasTurkishSubtitle must NOT be called for non-.mkv/.mp4 files
	vp.AssertNotCalled(t, "HasTargetSubtitle")

	svc := newScanSvc(subRepo, vp, &testmocks.MockTaskQueue{}, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_SkipsIfHasTurkishSubtitle(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "movie.mkv")

	subRepo := &testmocks.MockSubtitleRepository{}
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer vp.AssertExpectations(t)

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(true, nil)
	// EnsureEngSubtitle must NOT be called
	vp.AssertNotCalled(t, "EnsureEngSubtitle")
	queue.AssertNotCalled(t, "Enqueue")

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_SkipsExistingQueuedSubtitle(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "show.S01E01.mkv")
	engPath := filepath.Join(dir, "show.S01E01.eng.srt")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	existing := makeSubtitleAt(t, engPath, valueobject.StatusQueued)

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(false, nil)
	vp.On("EnsureEngSubtitle", mock.Anything, videoPath).Return(engPath, nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(existing, nil)
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	queue.AssertNotCalled(t, "Enqueue")

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_SkipsExistingDoneSubtitle(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "film.mkv")
	engPath := filepath.Join(dir, "film.eng.srt")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	existing := makeSubtitleAt(t, engPath, valueobject.StatusDone)

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(false, nil)
	vp.On("EnsureEngSubtitle", mock.Anything, videoPath).Return(engPath, nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(existing, nil)
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	queue.AssertNotCalled(t, "Enqueue")

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_SkipsRelocatedViaSxxExx(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "S03E07.mkv")
	engPath := filepath.Join(dir, "S03E07.eng.srt")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	// Exact path not found, but SxxExx match found
	notFoundErr := errors.New("not found")
	oldSubtitle := makeSubtitleAt(t, "/old/path/S03E07.eng.srt", valueobject.StatusDone)

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(false, nil)
	vp.On("EnsureEngSubtitle", mock.Anything, videoPath).Return(engPath, nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(nil, notFoundErr)
	subRepo.On("FindBySxxExx", mock.Anything, 3, 7).Return([]*entity.Subtitle{oldSubtitle}, nil)
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	queue.AssertNotCalled(t, "Enqueue")

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_EnqueueAndSavesNewSubtitle(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "new.S01E01.mkv")
	engPath := filepath.Join(dir, "new.S01E01.eng.srt")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)
	defer queue.AssertExpectations(t)

	notFoundErr := errors.New("not found")

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(false, nil)
	vp.On("EnsureEngSubtitle", mock.Anything, videoPath).Return(engPath, nil)
	subRepo.On("FindByPath", mock.Anything, engPath).Return(nil, notFoundErr)
	subRepo.On("FindBySxxExx", mock.Anything, 1, 1).Return([]*entity.Subtitle{}, nil)
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	queue.On("Enqueue", mock.Anything, "translate_srt", mock.MatchedBy(func(task port.TranslateTask) bool {
		return task.EngPath == engPath && task.VideoPath == videoPath && task.TargetLanguage == "tr"
	})).Return(nil)
	subRepo.On("Save", mock.Anything, mock.AnythingOfType("*entity.Subtitle")).Return(nil)

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)

	queue.AssertCalled(t, "Enqueue", mock.Anything, "translate_srt", mock.AnythingOfType("port.TranslateTask"))
	subRepo.AssertCalled(t, "Save", mock.Anything, mock.AnythingOfType("*entity.Subtitle"))
}

func TestScanningService_Scan_BothMkvAndMp4_Processed(t *testing.T) {
	dir := t.TempDir()
	mkvPath := makeTempFile(t, dir, "movie.mkv")
	mp4Path := makeTempFile(t, dir, "episode.mp4")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer subRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	// Both skipped because Turkish sub already exists
	vp.On("HasTargetSubtitle", mock.Anything, mkvPath, "tur").Return(true, nil)
	vp.On("HasTargetSubtitle", mock.Anything, mp4Path, "tur").Return(true, nil)
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}

func TestScanningService_Scan_UsesWatchDirRepo(t *testing.T) {
	dir := t.TempDir()
	makeTempFile(t, dir, "film.mkv")

	wdRepo := &testmocks.MockWatchDirRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer wdRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	// Repo returns the temp dir
	wdRepo.On("FindEnabled", mock.Anything).Return([]string{dir}, nil)
	vp.On("HasTargetSubtitle", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true, nil)

	subRepo := &testmocks.MockSubtitleRepository{}
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	// watchDirs (config) is empty — must use repo result
	svc := newScanSvc(subRepo, vp, &testmocks.MockTaskQueue{}, []string{}, wdRepo)
	err := svc.Scan(context.Background())
	require.NoError(t, err)

	wdRepo.AssertCalled(t, "FindEnabled", mock.Anything)
}

func TestScanningService_Scan_FallsBackToConfigDirs(t *testing.T) {
	dir := t.TempDir()
	makeTempFile(t, dir, "film.mkv")

	wdRepo := &testmocks.MockWatchDirRepository{}
	vp := &testmocks.MockVideoProcessor{}
	defer wdRepo.AssertExpectations(t)
	defer vp.AssertExpectations(t)

	// Repo returns empty list → fallback to config dirs
	wdRepo.On("FindEnabled", mock.Anything).Return([]string{}, nil)
	vp.On("HasTargetSubtitle", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string")).Return(true, nil)

	subRepo2 := &testmocks.MockSubtitleRepository{}
	subRepo2.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)
	svc := newScanSvc(subRepo2, vp, &testmocks.MockTaskQueue{}, []string{dir}, wdRepo)
	err := svc.Scan(context.Background())
	require.NoError(t, err)

	vp.AssertCalled(t, "HasTargetSubtitle", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("string"))
}

func TestScanningService_Scan_SkipsWhenEngSubtitleExtractionFails(t *testing.T) {
	dir := t.TempDir()
	videoPath := makeTempFile(t, dir, "film.mkv")

	subRepo := &testmocks.MockSubtitleRepository{}
	vp := &testmocks.MockVideoProcessor{}
	queue := &testmocks.MockTaskQueue{}
	defer vp.AssertExpectations(t)

	vp.On("HasTargetSubtitle", mock.Anything, videoPath, "tur").Return(false, nil)
	vp.On("EnsureEngSubtitle", mock.Anything, videoPath).Return("", errors.New("no eng stream"))
	queue.AssertNotCalled(t, "Enqueue")
	subRepo.AssertNotCalled(t, "Save")
	subRepo.On("FindAll", mock.Anything).Return([]*entity.Subtitle{}, nil)

	svc := newScanSvc(subRepo, vp, queue, []string{dir}, nil)
	err := svc.Scan(context.Background())
	require.NoError(t, err)
}
