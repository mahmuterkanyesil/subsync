package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/exception"
)

func TestNewWatchDir_Valid(t *testing.T) {
	// t.TempDir() gives a real, existing absolute directory on the current OS
	existingDir := t.TempDir()

	tests := []struct {
		name string
		path string
	}{
		// Real existing directory — os.Stat succeeds and it is a directory
		{"existing temp dir", existingDir},
		// Non-existent Windows drive-letter path — accepted on Linux for cross-env support
		{"windows drive letter nonexistent", `C:\nonexistent\subsync_test_path`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wd, err := entity.NewWatchDir(tt.path)
			require.NoError(t, err)
			assert.True(t, wd.IsEnabled(), "new watch dir should be enabled by default")
			assert.False(t, wd.CreatedAt().IsZero())
		})
	}
}

func TestNewWatchDir_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		errFragment string
	}{
		{"empty path", "", "cannot be empty"},
		{"relative path", "subdir/movies", "must be absolute"},
		{"dot-relative path", "./movies", "must be absolute"},
		{"parent-relative path", "../movies", "must be absolute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := entity.NewWatchDir(tt.path)
			require.Error(t, err)

			var domainErr *exception.InvalidSubtitleException
			require.True(t, errors.As(err, &domainErr))
			assert.Contains(t, domainErr.Message, tt.errFragment)
		})
	}
}

func TestWatchDir_Enable_Disable_Toggle(t *testing.T) {
	wd := entity.RestoreWatchDir(1, "/media/tv", false, time.Now())

	assert.False(t, wd.IsEnabled())

	wd.Enable()
	assert.True(t, wd.IsEnabled())

	wd.Disable()
	assert.False(t, wd.IsEnabled())

	wd.Toggle() // false → true
	assert.True(t, wd.IsEnabled())

	wd.Toggle() // true → false
	assert.False(t, wd.IsEnabled())
}

func TestRestoreWatchDir_RoundTrip(t *testing.T) {
	createdAt := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	wd := entity.RestoreWatchDir(42, "/media/movies", true, createdAt)

	assert.Equal(t, 42, wd.ID())
	assert.Equal(t, "/media/movies", wd.Path())
	assert.True(t, wd.IsEnabled())
	assert.Equal(t, createdAt, wd.CreatedAt())
}
