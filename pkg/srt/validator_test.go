package srt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/srt"
)

func TestIsTurkish(t *testing.T) {
	tests := []struct {
		name   string
		blocks []valueobject.SRTBlock
		want   bool
	}{
		{
			name:   "empty blocks returns false",
			blocks: []valueobject.SRTBlock{},
			want:   false,
		},
		{
			name: "Turkish text block returns true",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Bugün ğüzel bir gün"},
			},
			want: true,
		},
		{
			name: "English text block returns false",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "He went to the store"},
			},
			want: false,
		},
		{
			name: "multiple Turkish blocks returns true",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Merhaba"},
				{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "nasıl"},
				{Index: 3, Timestamp: "00:00:05,000 --> 00:00:06,000", Text: "şimdi"},
			},
			want: true,
		},
		{
			name: "block with no Turkish chars returns false",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Hello world"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srt.IsTurkish(tt.blocks)
			assert.Equal(t, tt.want, got)
		})
	}
}
