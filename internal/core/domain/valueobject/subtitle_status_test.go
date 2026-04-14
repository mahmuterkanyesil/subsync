package valueobject_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"subsync/internal/core/domain/valueobject"
)

func TestSubtitleStatus_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   valueobject.SubtitleStatus
		to     valueobject.SubtitleStatus
		wantOk bool
	}{
		// From Queued
		{"queued→done", valueobject.StatusQueued, valueobject.StatusDone, true},
		{"queued→error", valueobject.StatusQueued, valueobject.StatusError, true},
		{"queued→quota_exhausted", valueobject.StatusQueued, valueobject.StatusQuotaExhausted, true},
		{"queued→embedded blocked", valueobject.StatusQueued, valueobject.StatusEmbedded, false},
		{"queued→embed_failed blocked", valueobject.StatusQueued, valueobject.StatusEmbedFailed, false},

		// From Done
		{"done→embedded", valueobject.StatusDone, valueobject.StatusEmbedded, true},
		{"done→queued", valueobject.StatusDone, valueobject.StatusQueued, true},
		{"done→embed_failed", valueobject.StatusDone, valueobject.StatusEmbedFailed, true},
		{"done→error blocked", valueobject.StatusDone, valueobject.StatusError, false},
		{"done→quota_exhausted blocked", valueobject.StatusDone, valueobject.StatusQuotaExhausted, false},

		// From Error
		{"error→queued", valueobject.StatusError, valueobject.StatusQueued, true},
		{"error→done blocked", valueobject.StatusError, valueobject.StatusDone, false},
		{"error→embedded blocked", valueobject.StatusError, valueobject.StatusEmbedded, false},

		// From QuotaExhausted
		{"quota_exhausted→queued", valueobject.StatusQuotaExhausted, valueobject.StatusQueued, true},
		{"quota_exhausted→done blocked", valueobject.StatusQuotaExhausted, valueobject.StatusDone, false},
		{"quota_exhausted→error blocked", valueobject.StatusQuotaExhausted, valueobject.StatusError, false},

		// From Embedded
		{"embedded→done", valueobject.StatusEmbedded, valueobject.StatusDone, true},
		{"embedded→queued blocked", valueobject.StatusEmbedded, valueobject.StatusQueued, false},
		{"embedded→error blocked", valueobject.StatusEmbedded, valueobject.StatusError, false},

		// From EmbedFailed
		{"embed_failed→done", valueobject.StatusEmbedFailed, valueobject.StatusDone, true},
		{"embed_failed→queued blocked", valueobject.StatusEmbedFailed, valueobject.StatusQueued, false},
		{"embed_failed→error blocked", valueobject.StatusEmbedFailed, valueobject.StatusError, false},

		// Unknown status
		{"unknown→queued always false", valueobject.SubtitleStatus("unknown"), valueobject.StatusQueued, false},
		{"unknown→done always false", valueobject.SubtitleStatus("unknown"), valueobject.StatusDone, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.from.CanTransitionTo(tt.to)
			assert.Equal(t, tt.wantOk, got)
		})
	}
}
