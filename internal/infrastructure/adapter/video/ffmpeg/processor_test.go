package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── helpers ────────────────────────────────────────────────────────────────

func makeStream(lang, title string) subtitleStream {
	s := subtitleStream{}
	s.Tags.Language = lang
	s.Tags.Title = title
	return s
}

func writeSRT(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// srtContent builds a minimal valid SRT with n blocks.
func srtContent(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		sb.WriteString("1\n00:00:01,000 --> 00:00:02,000\nLine\n\n")
	}
	return sb.String()
}

// ─── orderedStreamIndices ────────────────────────────────────────────────────

func TestOrderedStreamIndices_EmptyInput(t *testing.T) {
	assert.Empty(t, orderedStreamIndices(nil))
}

func TestOrderedStreamIndices_EnglishFirst(t *testing.T) {
	streams := []subtitleStream{
		makeStream("fre", "French"),
		makeStream("eng", "English"),
		makeStream("ger", "German"),
	}
	got := orderedStreamIndices(streams)
	// English stream (index 1) must come before French (0) and German (2)
	require.Len(t, got, 3)
	assert.Equal(t, 1, got[0], "English stream must be first")
	assert.ElementsMatch(t, []int{0, 2}, got[1:])
}

func TestOrderedStreamIndices_MultipleEnglishFirst(t *testing.T) {
	streams := []subtitleStream{
		makeStream("fre", "French"),
		makeStream("en", "English Regular"),
		makeStream("eng", "English Full"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 3)
	assert.Equal(t, 1, got[0])
	assert.Equal(t, 2, got[1])
	assert.Equal(t, 0, got[2])
}

func TestOrderedStreamIndices_SkipsForced(t *testing.T) {
	streams := []subtitleStream{
		makeStream("eng", "Forced"),
		makeStream("eng", "English Full"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0], "forced stream must be excluded")
}

func TestOrderedStreamIndices_SkipsSigns(t *testing.T) {
	streams := []subtitleStream{
		makeStream("eng", "Signs & Songs"),
		makeStream("fre", "French"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0])
}

func TestOrderedStreamIndices_SkipsSDH(t *testing.T) {
	streams := []subtitleStream{
		makeStream("eng", "English SDH"),
		makeStream("eng", "English"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0])
}

func TestOrderedStreamIndices_SkipsSongs(t *testing.T) {
	streams := []subtitleStream{
		makeStream("jpn", "Songs"),
		makeStream("jpn", "Dialog"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0])
}

func TestOrderedStreamIndices_AllSkipped(t *testing.T) {
	streams := []subtitleStream{
		makeStream("eng", "Forced"),
		makeStream("eng", "Signs"),
	}
	assert.Empty(t, orderedStreamIndices(streams))
}

func TestOrderedStreamIndices_NoEnglish_AllOthersIncluded(t *testing.T) {
	streams := []subtitleStream{
		makeStream("fre", "French"),
		makeStream("spa", "Spanish"),
	}
	got := orderedStreamIndices(streams)
	assert.Equal(t, []int{0, 1}, got)
}

func TestOrderedStreamIndices_SkipTitleCaseInsensitive(t *testing.T) {
	streams := []subtitleStream{
		makeStream("eng", "FORCED"),
		makeStream("eng", "Full"),
	}
	got := orderedStreamIndices(streams)
	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0])
}

// ─── parseSRTFileBlockCount ──────────────────────────────────────────────────

func TestParseSRTFileBlockCount_MissingFile(t *testing.T) {
	assert.Equal(t, 0, parseSRTFileBlockCount("/nonexistent/path.srt"))
}

func TestParseSRTFileBlockCount_ValidBlocks(t *testing.T) {
	dir := t.TempDir()
	path := writeSRT(t, dir, "sub.srt", srtContent(10))
	assert.Equal(t, 10, parseSRTFileBlockCount(path))
}

func TestParseSRTFileBlockCount_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeSRT(t, dir, "empty.srt", "")
	assert.Equal(t, 0, parseSRTFileBlockCount(path))
}

func TestParseSRTFileBlockCount_SmallButValid(t *testing.T) {
	// Regression: a file under 10KB must not be rejected when it has enough blocks.
	// This was the Dr. STONE S04E27 scenario: dialog-light episode with valid subs.
	dir := t.TempDir()
	path := writeSRT(t, dir, "small.srt", srtContent(minSRTBlocks))
	n := parseSRTFileBlockCount(path)
	assert.GreaterOrEqual(t, n, minSRTBlocks, "file with exactly minSRTBlocks must pass")

	info, _ := os.Stat(path)
	assert.Less(t, info.Size(), int64(10*1024), "fixture must be under 10KB to validate the regression case")
}

func TestParseSRTFileBlockCount_BelowMinThreshold(t *testing.T) {
	dir := t.TempDir()
	path := writeSRT(t, dir, "tiny.srt", srtContent(minSRTBlocks-1))
	assert.Less(t, parseSRTFileBlockCount(path), minSRTBlocks)
}

func TestParseSRTFileBlockCount_NonSRTContent(t *testing.T) {
	dir := t.TempDir()
	path := writeSRT(t, dir, "junk.srt", "not a subtitle file\nrandom content\n")
	assert.Equal(t, 0, parseSRTFileBlockCount(path))
}

// ─── minSRTBlocks constant ───────────────────────────────────────────────────

func TestMinSRTBlocks_Value(t *testing.T) {
	// Ensure the constant stays at a sensible value — not zero (no guard) or
	// too large (rejects short but valid episodes).
	assert.GreaterOrEqual(t, minSRTBlocks, 3)
	assert.LessOrEqual(t, minSRTBlocks, 20)
}
