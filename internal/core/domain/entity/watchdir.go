package entity

import (
	"os"
	"path/filepath"
	"subsync/internal/core/domain/exception"
	"time"
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

	// Require absolute path to avoid ambiguous relative entries
	if !filepath.IsAbs(p) {
		return nil, &exception.InvalidSubtitleException{Message: "watch directory path must be absolute"}
	}

	// Optional: check that path exists and is a directory
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
