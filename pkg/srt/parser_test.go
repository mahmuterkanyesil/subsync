package srt_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/valueobject"
	"subsync/pkg/srt"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []valueobject.SRTBlock
	}{
		{
			name:  "empty string returns empty slice",
			input: "",
			want:  []valueobject.SRTBlock{},
		},
		{
			name:  "single valid block",
			input: "1\n00:00:01,000 --> 00:00:02,000\nHello world\n\n",
			want: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Hello world"},
			},
		},
		{
			name: "two consecutive blocks",
			input: "1\n00:00:01,000 --> 00:00:02,000\nFirst line\n\n" +
				"2\n00:00:03,000 --> 00:00:04,000\nSecond line\n\n",
			want: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "First line"},
				{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "Second line"},
			},
		},
		{
			name:  "CRLF line endings are normalized",
			input: "1\r\n00:00:01,000 --> 00:00:02,000\r\nText here\r\n\r\n",
			want: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Text here"},
			},
		},
		{
			name:  "non-numeric index defaults to zero",
			input: "abc\n00:00:01,000 --> 00:00:02,000\nText here\n\n",
			want: []valueobject.SRTBlock{
				{Index: 0, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Text here"},
			},
		},
		{
			name:  "block with fewer than 3 lines is skipped",
			input: "1\n00:00:01,000 --> 00:00:02,000\n",
			want:  []valueobject.SRTBlock{},
		},
		{
			name:  "multi-line text joined with newline",
			input: "1\n00:00:01,000 --> 00:00:02,000\nLine A\nLine B\n\n",
			want: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Line A\nLine B"},
			},
		},
		{
			name:  "only index and timestamp (no text) — skipped",
			input: "1\n00:00:01,000 --> 00:00:02,000",
			want:  []valueobject.SRTBlock{},
		},
		{
			name: "mixed valid and invalid blocks",
			input: "1\n00:00:01,000 --> 00:00:02,000\nGood block\n\n" +
				"only-one-line\n\n" +
				"2\n00:00:03,000 --> 00:00:04,000\nAnother good\n\n",
			want: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Good block"},
				{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "Another good"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srt.Parse(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormat(t *testing.T) {
	tests := []struct {
		name   string
		blocks []valueobject.SRTBlock
		want   string
	}{
		{
			name:   "empty blocks returns empty string",
			blocks: []valueobject.SRTBlock{},
			want:   "",
		},
		{
			name: "single block formatted correctly",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Hello"},
			},
			want: "1\n00:00:01,000 --> 00:00:02,000\nHello\n\n",
		},
		{
			name: "two blocks formatted correctly",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "First"},
				{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "Second"},
			},
			want: "1\n00:00:01,000 --> 00:00:02,000\nFirst\n\n" +
				"2\n00:00:03,000 --> 00:00:04,000\nSecond\n\n",
		},
		{
			name: "multiline text preserved in format",
			blocks: []valueobject.SRTBlock{
				{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Line A\nLine B"},
			},
			want: "1\n00:00:01,000 --> 00:00:02,000\nLine A\nLine B\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srt.Format(tt.blocks)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseThenFormat_RoundTrip(t *testing.T) {
	inputs := []string{
		"1\n00:00:01,000 --> 00:00:02,000\nHello\n\n",
		"1\n00:00:01,000 --> 00:00:02,000\nFirst\n\n2\n00:00:03,000 --> 00:00:04,000\nSecond\n\n",
		"1\n00:00:01,000 --> 00:00:02,000\nLine A\nLine B\n\n",
	}

	for _, input := range inputs {
		blocks := srt.Parse(input)
		require.NotEmpty(t, blocks)

		reformatted := srt.Format(blocks)
		reparsed := srt.Parse(reformatted)

		assert.Equal(t, blocks, reparsed, "round-trip: Parse(Format(blocks)) should equal original")
	}
}
