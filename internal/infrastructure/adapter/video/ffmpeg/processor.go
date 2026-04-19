package ffmpeg

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"subsync/internal/core/application/port"
	"subsync/pkg/logger"
	"subsync/pkg/srt"
)

var engSrtRegex = regexp.MustCompile(`(?i)\.en[^a-z]|\.eng[^a-z]`)

// minSRTBlocks is the minimum number of parsed SRT blocks required for a
// subtitle file to be considered usable. Replaces the old 10KB size check,
// which rejected valid files from dialog-light episodes.
const minSRTBlocks = 5

var skipTitles = []string{"forced", "signs", "songs", "sdh"}

type subtitleStream struct {
	Index int
	Tags  struct {
		Language string `json:"language"`
		Title    string `json:"title"`
	} `json:"tags"`
}

type ffprobeOutput struct {
	Streams []subtitleStream `json:"streams"`
}

type FFmpegProcessor struct{}

func NewFFmpegProcessor() *FFmpegProcessor {
	return &FFmpegProcessor{}
}

// orderedStreamIndices returns subtitle stream indices in preference order:
// English-tagged non-skip streams first, then all remaining non-skip streams.
// Streams tagged forced/signs/songs/sdh are excluded entirely.
func orderedStreamIndices(streams []subtitleStream) []int {
	var eng, other []int
	for i, s := range streams {
		title := strings.ToLower(s.Tags.Title)
		skip := false
		for _, bad := range skipTitles {
			if strings.Contains(title, bad) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		lang := strings.ToLower(s.Tags.Language)
		if lang == "eng" || lang == "en" {
			eng = append(eng, i)
		} else {
			other = append(other, i)
		}
	}
	return append(eng, other...)
}

// parseSRTFile reads path and returns parsed blocks. Returns (nil, false) if
// the file cannot be read.
func parseSRTFile(path string) ([]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	blocks := srt.Parse(string(data))
	texts := make([]string, len(blocks))
	for i := range blocks {
		texts[i] = blocks[i].Text
	}
	return texts, true
}

// parseSRTFileBlockCount returns the number of SRT blocks in path.
// Returns 0 if the file cannot be read or parsed.
func parseSRTFileBlockCount(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return len(srt.Parse(string(data)))
}

func (f *FFmpegProcessor) HasTargetSubtitle(ctx context.Context, videoPath, langFFmpegCode string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		logger.Warn("ffprobe failed: %s — %v", filepath.Base(videoPath), err)
		return false, err
	}
	output := strings.ToLower(string(out))
	return strings.Contains(output, "\""+strings.ToLower(langFFmpegCode)+"\""), nil
}

func (f *FFmpegProcessor) EnsureEngSubtitle(ctx context.Context, videoPath string) (string, error) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))

	// Step 1: Hardcoded sidecar candidates — validate by block count, not size.
	for _, c := range []string{base + ".eng.srt", base + ".en.srt", base + ".english.srt"} {
		if n := parseSRTFileBlockCount(c); n >= minSRTBlocks {
			return c, nil
		}
		if _, err := os.Stat(c); err == nil {
			_ = os.Remove(c) // exists but unusable — delete so we re-extract
		}
	}

	// Step 2: Regex scan directory for eng-named srt files.
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(videoPath), "*.srt"))
	for _, m := range matches {
		if engSrtRegex.MatchString(filepath.Base(m)) {
			if parseSRTFileBlockCount(m) >= minSRTBlocks {
				return m, nil
			}
		}
	}

	// Step 3: ffprobe — discover all subtitle streams.
	streams, err := f.probeSubtitleStreams(ctx, videoPath)
	if err != nil {
		return "", err
	}

	indices := orderedStreamIndices(streams)
	if len(indices) == 0 {
		return "", port.ErrNoEngStream
	}

	// Step 4: Try each stream in preference order; keep first with enough blocks.
	engPath := base + ".eng.srt"
	for _, idx := range indices {
		tmpPath := fmt.Sprintf("%s.stream%d.tmp.srt", base, idx)
		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath,
			"-map", fmt.Sprintf("0:s:%d", idx),
			"-c:s", "srt",
			tmpPath, "-y",
		)
		logger.Info("extract sub stream %d: %s", idx, filepath.Base(videoPath))
		if err := cmd.Run(); err != nil {
			logger.Warn("stream %d extract failed: %s — %v", idx, filepath.Base(videoPath), err)
			_ = os.Remove(tmpPath)
			continue
		}
		n := parseSRTFileBlockCount(tmpPath)
		if n < minSRTBlocks {
			logger.Warn("stream %d too few blocks (%d/%d): %s", idx, n, minSRTBlocks, filepath.Base(videoPath))
			_ = os.Remove(tmpPath)
			continue
		}
		if err := os.Rename(tmpPath, engPath); err != nil {
			_ = os.Remove(tmpPath)
			return "", err
		}
		return engPath, nil
	}

	return "", fmt.Errorf("no subtitle stream produced %d+ blocks: %w", minSRTBlocks, port.ErrNoEngStream)
}

func (f *FFmpegProcessor) probeSubtitleStreams(ctx context.Context, videoPath string) ([]subtitleStream, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil, fmt.Errorf("ffprobe parse failed: %w", err)
	}
	return probe.Streams, nil
}

func (f *FFmpegProcessor) countSubtitleStreams(ctx context.Context, videoPath string) int {
	streams, err := f.probeSubtitleStreams(ctx, videoPath)
	if err != nil {
		return 0
	}
	return len(streams)
}

func (f *FFmpegProcessor) probeOK(ctx context.Context, path string) bool {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=nw=1:nk=1",
		path,
	)
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func (f *FFmpegProcessor) EmbedSubtitle(ctx context.Context, videoPath, srtPath, langFFmpegCode, langNameEN string) error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return port.ErrFFmpegNotFound
	}
	if _, err := os.Stat(videoPath); err != nil {
		return port.ErrVideoNotFound
	}
	if _, err := os.Stat(srtPath); err != nil {
		return port.ErrTrSrtNotFound
	}

	origInfo, err := os.Stat(videoPath)
	if err != nil {
		return port.ErrVideoNotFound
	}

	ext := strings.ToLower(filepath.Ext(videoPath))
	tmpPath := videoPath + ".tmp" + ext

	var cmd *exec.Cmd
	if ext == ".mkv" {
		existingSubs := f.countSubtitleStreams(ctx, videoPath)
		newIndex := fmt.Sprintf("%d", existingSubs)
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath, "-i", srtPath,
			"-map", "0", "-map", "1:0",
			"-c:v", "copy", "-c:a", "copy", "-c:s", "copy",
			"-metadata:s:s:"+newIndex, "language="+langFFmpegCode,
			"-metadata:s:s:"+newIndex, "title="+langNameEN+" (Subsync)",
			tmpPath, "-y",
		)
	} else {
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath, "-i", srtPath,
			"-map", "0:v?", "-map", "0:a?", "-map", "1:0",
			"-c:v", "copy", "-c:a", "copy", "-c:s", "mov_text",
			"-metadata:s:s:0", "language="+langFFmpegCode,
			"-metadata:s:s:0", "title="+langNameEN+" (Subsync)",
			tmpPath, "-y",
		)
	}

	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: %v", port.ErrFFmpegFailed, err)
	}

	tmpInfo, err := os.Stat(tmpPath)
	if err != nil || tmpInfo.Size() < origInfo.Size()*9/10 {
		_ = os.Remove(tmpPath)
		return port.ErrOutputTooSmall
	}

	if !f.probeOK(ctx, tmpPath) {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("%w: container integrity check failed", port.ErrFFmpegFailed)
	}

	return os.Rename(tmpPath, videoPath)
}
