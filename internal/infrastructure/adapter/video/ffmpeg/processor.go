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
)

var engSrtRegex = regexp.MustCompile(`(?i)\.en[^a-z]|\.eng[^a-z]`)

type ffprobeOutput struct {
	Streams []struct {
		Index int `json:"index"`
		Tags  struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

type FFmpegProcessor struct{}

func NewFFmpegProcessor() *FFmpegProcessor {
	return &FFmpegProcessor{}
}

func isSizeValid(path string, minBytes int64) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() >= minBytes
}

func (f *FFmpegProcessor) HasTurkishSubtitle(ctx context.Context, videoPath string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	output := strings.ToLower(string(out))
	return strings.Contains(output, "\"tur\"") || strings.Contains(output, "turkish"), nil
}

func (f *FFmpegProcessor) EnsureEngSubtitle(ctx context.Context, videoPath string) (string, error) {
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))

	// Step 1: Hardcoded candidates
	for _, c := range []string{base + ".eng.srt", base + ".en.srt", base + ".english.srt"} {
		if info, err := os.Stat(c); err == nil {
			if info.Size() >= 10*1024 {
				return c, nil
			}
			_ = os.Remove(c) // too small — re-extract
		}
	}

	// Step 2: Regex scan directory
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(videoPath), "*.srt"))
	for _, m := range matches {
		if engSrtRegex.MatchString(filepath.Base(m)) && isSizeValid(m, 10*1024) {
			return m, nil
		}
	}

	// Step 3: ffprobe language-aware stream selection
	streamIndex, err := f.findEngSubtitleStream(ctx, videoPath)
	if err != nil {
		return "", err
	}

	// Step 4: Extract stream
	engPath := base + ".eng.srt"
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:s:%d", streamIndex),
		"-c:s", "srt",
		engPath,
		"-y",
	)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("subtitle extraction failed: %w", err)
	}

	if !isSizeValid(engPath, 10*1024) {
		_ = os.Remove(engPath)
		return "", fmt.Errorf("extracted subtitle too small (< 10KB)")
	}

	return engPath, nil
}

func (f *FFmpegProcessor) findEngSubtitleStream(ctx context.Context, videoPath string) (int, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, fmt.Errorf("ffprobe parse failed: %w", err)
	}

	skipTitles := []string{"forced", "signs", "songs", "sdh"}

	for i, s := range probe.Streams {
		lang := strings.ToLower(s.Tags.Language)
		title := strings.ToLower(s.Tags.Title)

		if lang != "eng" && lang != "en" {
			continue
		}
		skip := false
		for _, bad := range skipTitles {
			if strings.Contains(title, bad) {
				skip = true
				break
			}
		}
		if !skip {
			return i, nil
		}
	}

	// Fall back to first stream if no language-tagged stream found
	if len(probe.Streams) > 0 {
		return 0, nil
	}
	return 0, port.ErrNoEngStream
}

func (f *FFmpegProcessor) countSubtitleStreams(ctx context.Context, videoPath string) int {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		videoPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0
	}
	return len(probe.Streams)
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

func (f *FFmpegProcessor) EmbedSubtitle(ctx context.Context, videoPath string, srtPath string) error {
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
			"-metadata:s:s:"+newIndex, "language=tur",
			"-metadata:s:s:"+newIndex, "title=Turkish (Subsync)",
			tmpPath, "-y",
		)
	} else {
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath, "-i", srtPath,
			"-map", "0:v?", "-map", "0:a?", "-map", "1:0",
			"-c:v", "copy", "-c:a", "copy", "-c:s", "mov_text",
			"-metadata:s:s:0", "language=tur",
			"-metadata:s:s:0", "title=Turkish (Subsync)",
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
