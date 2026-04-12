package entity

import (
	"os"
	"path/filepath"
	"subsync/internal/core/domain/exception"
	"time"
	"unicode"
)

type WatchDir struct {
	id        int
	path      string
	enabled   bool
	createdAt time.Time
}

// NewWatchDir validates and normalizes the provided path.
// It requires an absolute path and returns an error for empty or relative paths.
func NewWatchDir(path string) (*WatchDir, error) {
	if path == "" {
		return nil, &exception.InvalidSubtitleException{Message: "watch directory path cannot be empty"}
	}

	// Normalize separators and clean the path
	p := filepath.Clean(filepath.FromSlash(path))

	// Require absolute path to avoid ambiguous relative entries.
	// Accept Windows drive-letter paths (e.g. C:\...) even when running on non-Windows hosts.
	isAbs := filepath.IsAbs(p)
	if !isAbs {
		// detect Windows drive-letter absolute paths like C:\Users\...
		if len(p) >= 2 && p[1] == ':' {
			r := rune(p[0])
			if unicode.IsLetter(r) {
				isAbs = true
			}
		}
	}
	if !isAbs {
		return nil, &exception.InvalidSubtitleException{Message: "watch directory path must be absolute"}
	}

	// Check existence only if the path actually exists in the running environment.
	// If it doesn't exist (common when adding host Windows paths from a Linux container),
	// accept the entry but don't error — this lets users add host paths that aren't visible
	// to the running process.
	if fi, err := os.Stat(p); err == nil {
		if !fi.IsDir() {
			return nil, &exception.InvalidSubtitleException{Message: "watch directory path is not a directory"}
		}
	}

	return &WatchDir{
		path:      p,
		enabled:   true,
		createdAt: time.Now(),
	}, nil
}

func RestoreWatchDir(id int, path string, enabled bool, createdAt time.Time) *WatchDir {
	return &WatchDir{id: id, path: path, enabled: enabled, createdAt: createdAt}
}

func (w *WatchDir) ID() int               { return w.id }
func (w *WatchDir) Path() string          { return w.path }
func (w *WatchDir) IsEnabled() bool       { return w.enabled }
func (w *WatchDir) CreatedAt() time.Time  { return w.createdAt }

func (w *WatchDir) Enable()  { w.enabled = true }
func (w *WatchDir) Disable() { w.enabled = false }
func (w *WatchDir) Toggle()  { w.enabled = !w.enabled }
