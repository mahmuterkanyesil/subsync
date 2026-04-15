package progress_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/progress"
)

func newStore(t *testing.T) *progress.FileProgressStore {
	t.Helper()
	return progress.NewFileProgressStore(t.TempDir())
}

func makeBlocks(n int) []valueobject.SRTBlock {
	blocks := make([]valueobject.SRTBlock, n)
	for i := range blocks {
		blocks[i] = valueobject.SRTBlock{
			Index:     i + 1,
			Timestamp: "00:00:01,000 --> 00:00:02,000",
			Text:      "Line text",
		}
	}
	return blocks
}

func TestFileProgressStore_SaveAndLoad(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	blocks := makeBlocks(2)

	err := store.Save(ctx, "/media/movie.eng.srt", blocks)
	require.NoError(t, err)

	loaded, found, err := store.Load(ctx, "/media/movie.eng.srt")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, blocks, loaded)
}

func TestFileProgressStore_Load_NotFound_ReturnsNilFalseNilErr(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	blocks, found, err := store.Load(ctx, "/media/nonexistent.eng.srt")

	assert.NoError(t, err)
	assert.False(t, found)
	assert.Nil(t, blocks)
}

func TestFileProgressStore_Clear_RemovesFile(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	blocks := makeBlocks(3)
	engPath := "/media/show.eng.srt"

	err := store.Save(ctx, engPath, blocks)
	require.NoError(t, err)

	err = store.Clear(ctx, engPath)
	require.NoError(t, err)

	_, found, err := store.Load(ctx, engPath)
	require.NoError(t, err)
	assert.False(t, found)
}

func TestFileProgressStore_Clear_NonExistent_NoError(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	err := store.Clear(ctx, "/media/nonexistent.eng.srt")
	assert.NoError(t, err)
}

func TestFileProgressStore_RoundTrip_LargeBlocks(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	blocks := makeBlocks(50)

	err := store.Save(ctx, "/tv/show.eng.srt", blocks)
	require.NoError(t, err)

	loaded, found, err := store.Load(ctx, "/tv/show.eng.srt")
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, blocks, loaded)
}

func TestFileProgressStore_Save_OverwritesPreviousProgress(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)
	engPath := "/media/ep.eng.srt"

	first := makeBlocks(2)
	err := store.Save(ctx, engPath, first)
	require.NoError(t, err)

	second := makeBlocks(5)
	err = store.Save(ctx, engPath, second)
	require.NoError(t, err)

	loaded, found, err := store.Load(ctx, engPath)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, second, loaded)
	assert.Len(t, loaded, 5)
}

func TestFileProgressStore_Load_InvalidJSON_ReturnsError(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store := progress.NewFileProgressStore(dir)
	engPath := "/media/corrupted.eng.srt"

	// Save valid data first so the store creates the hashed progress file
	require.NoError(t, store.Save(ctx, engPath, makeBlocks(1)))

	// Find the created file and overwrite with invalid JSON
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	progressFile := filepath.Join(dir, entries[0].Name())
	require.NoError(t, os.WriteFile(progressFile, []byte("not valid json {{"), 0644))

	_, _, err = store.Load(ctx, engPath)
	assert.Error(t, err)
}

func TestFileProgressStore_MultipleFiles_AreIsolated(t *testing.T) {
	ctx := context.Background()
	store := newStore(t)

	path1 := "/media/movie1.eng.srt"
	path2 := "/media/movie2.eng.srt"
	blocks1 := []valueobject.SRTBlock{{Index: 1, Timestamp: "ts1", Text: "Block 1"}}
	blocks2 := []valueobject.SRTBlock{{Index: 2, Timestamp: "ts2", Text: "Block 2"}}

	require.NoError(t, store.Save(ctx, path1, blocks1))
	require.NoError(t, store.Save(ctx, path2, blocks2))

	loaded1, found1, err := store.Load(ctx, path1)
	require.NoError(t, err)
	require.True(t, found1)
	assert.Equal(t, blocks1, loaded1)

	loaded2, found2, err := store.Load(ctx, path2)
	require.NoError(t, err)
	require.True(t, found2)
	assert.Equal(t, blocks2, loaded2)
}

func TestFileProgressStore_ProgressPath_DifferentDirs_AreIsolated(t *testing.T) {
	// Two paths with same basename but different directories must map to different files
	ctx := context.Background()
	store := newStore(t)

	path1 := "/dir1/movie.eng.srt"
	path2 := "/dir2/movie.eng.srt"
	blocks := []valueobject.SRTBlock{{Index: 1, Timestamp: "ts", Text: "Path1 content"}}

	require.NoError(t, store.Save(ctx, path1, blocks))

	// Loading path2 (same basename, different dir) must NOT find path1's progress
	_, found, err := store.Load(ctx, path2)
	require.NoError(t, err)
	assert.False(t, found, "different full paths should map to different progress files even if basename matches")
}
