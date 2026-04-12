package entity

import (
	"subsync/internal/core/domain/exception"
	"time"
)

type WatchDir struct {
	id        int
	path      string
	enabled   bool
	createdAt time.Time
}

func NewWatchDir(path string) (*WatchDir, error) {
	if path == "" {
		return nil, &exception.InvalidSubtitleException{Message: "watch directory path cannot be empty"}
	}
	return &WatchDir{
		path:      path,
		enabled:   true,
		createdAt: time.Now(),
	}, nil
}

func RestoreWatchDir(id int, path string, enabled bool, createdAt time.Time) *WatchDir {
	return &WatchDir{id: id, path: path, enabled: enabled, createdAt: createdAt}
}

func (w *WatchDir) ID() int           { return w.id }
func (w *WatchDir) Path() string      { return w.path }
func (w *WatchDir) IsEnabled() bool   { return w.enabled }
func (w *WatchDir) CreatedAt() time.Time { return w.createdAt }

func (w *WatchDir) Enable()  { w.enabled = true }
func (w *WatchDir) Disable() { w.enabled = false }
func (w *WatchDir) Toggle()  { w.enabled = !w.enabled }
