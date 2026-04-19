package gemini

import (
	"subsync/internal/core/application/port"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseResponse_AllBlocksPresent(t *testing.T) {
	g := NewGeminiTranslator()
	original := []port.SRTBlock{
		{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Hello"},
		{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "World"},
	}
	response := `
<b id="1">
1
00:00:01,000 --> 00:00:02,000
Merhaba
</b>
<b id="2">
2
00:00:03,000 --> 00:00:04,000
Dünya
</b>
`
	parsed, err := g.parseResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, parsed, 2)
	assert.Equal(t, "Merhaba", parsed[0].Text)
	assert.Equal(t, "Dünya", parsed[1].Text)
}

func TestParseResponse_MissingBlocks_FaultTolerance(t *testing.T) {
	g := NewGeminiTranslator()
	original := []port.SRTBlock{
		{Index: 1, Timestamp: "00:00:01,000 --> 00:00:02,000", Text: "Hello"},
		{Index: 2, Timestamp: "00:00:03,000 --> 00:00:04,000", Text: "World"},
		{Index: 3, Timestamp: "00:00:05,000 --> 00:00:06,000", Text: "Test"},
	}
	// LLM skips block 2
	response := `
<b id="1">
1
00:00:01,000 --> 00:00:02,000
Merhaba
</b>
<b id="3">
3
00:00:05,000 --> 00:00:06,000
Deneme
</b>
`
	parsed, err := g.parseResponse(response, original)
	assert.NoError(t, err)
	assert.Len(t, parsed, 3)
	assert.Equal(t, "Merhaba", parsed[0].Text)
	// Fallback to original text for missing block 2
	assert.Equal(t, "World", parsed[1].Text)
	assert.Equal(t, "Deneme", parsed[2].Text)
}
