package entity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/entity"
	"subsync/internal/core/domain/exception"
	"subsync/internal/core/domain/valueobject"
)

func validMediaInfo(t *testing.T) *valueobject.MediaInfo {
	t.Helper()
	mi, err := valueobject.NewMediaInfo(valueobject.MediaTypeMovie, "", 0, 0)
	require.NoError(t, err)
	return mi
}

func TestNewSubtitle_Valid(t *testing.T) {
	mi := validMediaInfo(t)
	s, err := entity.NewSubtitle(mi, "/media/movie.eng.srt")

	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, s.ID())
	assert.Equal(t, valueobject.StatusQueued, s.Status())
	assert.False(t, s.Embedded())
	assert.Equal(t, "/media/movie.eng.srt", s.EngPath())
	assert.Empty(t, s.LastError())
	assert.False(t, s.CreatedAt().IsZero())
	assert.False(t, s.UpdatedAt().IsZero())
}

func TestNewSubtitle_EmptyEngPath_ReturnsError(t *testing.T) {
	mi := validMediaInfo(t)
	_, err := entity.NewSubtitle(mi, "")

	require.Error(t, err)
	var domainErr *exception.InvalidSubtitleException
	assert.True(t, errors.As(err, &domainErr))
}

func TestRestoreSubtitle_EmptyEngPath_ReturnsError(t *testing.T) {
	mi := validMediaInfo(t)
	_, err := entity.RestoreSubtitle(
		uuid.New(), mi, "",
		valueobject.StatusQueued, "", false,
		time.Now(), time.Now(),
	)

	require.Error(t, err)
	var domainErr *exception.InvalidSubtitleException
	assert.True(t, errors.As(err, &domainErr))
}

func TestSubtitle_TransitionTo_Valid(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus valueobject.SubtitleStatus
		toStatus   valueobject.SubtitleStatus
	}{
		{"queued→done", valueobject.StatusQueued, valueobject.StatusDone},
		{"queued→error", valueobject.StatusQueued, valueobject.StatusError},
		{"queued→quota_exhausted", valueobject.StatusQueued, valueobject.StatusQuotaExhausted},
		{"done→embedded", valueobject.StatusDone, valueobject.StatusEmbedded},
		{"done→queued", valueobject.StatusDone, valueobject.StatusQueued},
		{"done→embed_failed", valueobject.StatusDone, valueobject.StatusEmbedFailed},
		{"error→queued", valueobject.StatusError, valueobject.StatusQueued},
		{"quota_exhausted→queued", valueobject.StatusQuotaExhausted, valueobject.StatusQueued},
		{"embedded→done", valueobject.StatusEmbedded, valueobject.StatusDone},
		{"embed_failed→done", valueobject.StatusEmbedFailed, valueobject.StatusDone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := validMediaInfo(t)
			s, err := entity.RestoreSubtitle(
				uuid.New(), mi, "/x.eng.srt",
				tt.fromStatus, "", false,
				time.Now(), time.Now(),
			)
			require.NoError(t, err)

			err = s.TransitionTo(tt.toStatus)
			require.NoError(t, err)
			assert.Equal(t, tt.toStatus, s.Status())
		})
	}
}

func TestSubtitle_TransitionTo_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus valueobject.SubtitleStatus
		toStatus   valueobject.SubtitleStatus
	}{
		{"queued→embedded blocked", valueobject.StatusQueued, valueobject.StatusEmbedded},
		{"done→error blocked", valueobject.StatusDone, valueobject.StatusError},
		{"embedded→queued blocked", valueobject.StatusEmbedded, valueobject.StatusQueued},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mi := validMediaInfo(t)
			s, err := entity.RestoreSubtitle(
				uuid.New(), mi, "/x.eng.srt",
				tt.fromStatus, "", false,
				time.Now(), time.Now(),
			)
			require.NoError(t, err)

			err = s.TransitionTo(tt.toStatus)
			require.Error(t, err)
			var transErr *exception.InvalidStatusTransitionException
			require.True(t, errors.As(err, &transErr))
			assert.Equal(t, string(tt.fromStatus), transErr.From)
			assert.Equal(t, string(tt.toStatus), transErr.To)
			// Status must not have changed
			assert.Equal(t, tt.fromStatus, s.Status())
		})
	}
}

func TestSubtitle_MarkEmbedded_SetsFlag(t *testing.T) {
	mi := validMediaInfo(t)
	s, err := entity.NewSubtitle(mi, "/x.eng.srt")
	require.NoError(t, err)

	assert.False(t, s.Embedded())
	s.MarkEmbedded()
	assert.True(t, s.Embedded())
}

func TestSubtitle_MarkUnembedded_ClearsFlags(t *testing.T) {
	mi := validMediaInfo(t)
	s, err := entity.RestoreSubtitle(
		uuid.New(), mi, "/x.eng.srt",
		valueobject.StatusEmbedded, "some error", true,
		time.Now(), time.Now(),
	)
	require.NoError(t, err)

	s.MarkUnembedded()

	assert.False(t, s.Embedded())
	assert.Empty(t, s.LastError())
}

func TestSubtitle_MarkError_SetsLastError(t *testing.T) {
	mi := validMediaInfo(t)
	s, err := entity.NewSubtitle(mi, "/x.eng.srt")
	require.NoError(t, err)

	s.MarkError(errors.New("translation failed"))
	assert.Equal(t, "translation failed", s.LastError())
}

func TestSubtitle_UpdatedAt_AdvancesOnMutation(t *testing.T) {
	mi := validMediaInfo(t)
	s, err := entity.NewSubtitle(mi, "/x.eng.srt")
	require.NoError(t, err)

	before := s.UpdatedAt()
	time.Sleep(2 * time.Millisecond)

	err = s.TransitionTo(valueobject.StatusDone)
	require.NoError(t, err)

	assert.True(t, s.UpdatedAt().After(before), "updatedAt should advance after mutation")
}
