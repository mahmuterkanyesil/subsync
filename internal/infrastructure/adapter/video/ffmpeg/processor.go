package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FFmpegProcessor struct{}

func NewFFmpegProcessor() *FFmpegProcessor {
	return &FFmpegProcessor{}
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
	// 1. .eng.srt dosyası var mı?
	base := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	candidates := []string{
		base + ".eng.srt",
		base + ".en.srt",
		base + ".english.srt",
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}

	// 2. Video container'dan çıkar
	engPath := base + ".eng.srt"
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", videoPath,
		"-map", "0:s:0",
		"-c:s", "srt",
		engPath,
		"-y",
	)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("subtitle extraction failed: %w", err)
	}

	return engPath, nil
}

func (f *FFmpegProcessor) EmbedSubtitle(ctx context.Context, videoPath string, srtPath string) error {
	ext := strings.ToLower(filepath.Ext(videoPath))
	tmpPath := videoPath + ".tmp" + ext

	var cmd *exec.Cmd

	if ext == ".mkv" {
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath,
			"-i", srtPath,
			"-map", "0",
			"-map", "1:0",
			"-c:v", "copy",
			"-c:a", "copy",
			"-c:s", "copy",
			"-metadata:s:s:0", "language=tur",
			"-metadata:s:s:0", "title=Turkish (Subsync)",
			tmpPath, "-y",
		)
	} else {
		cmd = exec.CommandContext(ctx, "ffmpeg",
			"-i", videoPath,
			"-i", srtPath,
			"-map", "0:v?",
			"-map", "0:a?",
			"-map", "1:0",
			"-c:v", "copy",
			"-c:a", "copy",
			"-c:s", "mov_text",
			"-metadata:s:s:0", "language=tur",
			"-metadata:s:s:0", "title=Turkish (Subsync)",
			tmpPath, "-y",
		)
	}

	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("ffmpeg failed: %w", err)
	}

	// Atomic replace
	return os.Rename(tmpPath, videoPath)
}
