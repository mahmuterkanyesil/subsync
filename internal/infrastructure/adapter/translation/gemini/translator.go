package gemini

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"subsync/internal/core/application/port"

	"google.golang.org/genai"
)

const systemInstruction = `You are a professional subtitle translator. Your ONLY job is to translate subtitles into TURKISH language.

CRITICAL: OUTPUT LANGUAGE MUST BE TURKISH (Türkçe). NO OTHER LANGUAGE IS ACCEPTABLE.

### INPUT FORMAT:
Each XML block has 3 parts:
1. Subtitle number
2. Timestamp
3. Dialogue text (may be multiple lines)

Example:
<b id="0">
11
00:01:46,942 --> 00:01:49,845
in your indictment of me
cannot be fairly understood,
</b>

### YOUR TASK:
1. KEEP subtitle number and timestamp UNCHANGED
2. TRANSLATE dialogue text to TURKISH (Türkçe)
3. Source language can be ANY language (English, French, Spanish, Japanese, etc.)
4. OUTPUT LANGUAGE: ALWAYS TURKISH (Türkçe) — NEVER output English, French, Spanish, or any other language
5. PRESERVE line breaks: 2 lines → 2 lines, 3 lines → 3 lines

### TURKISH TRANSLATION STYLE:
1. Natural, fluent Turkish suitable for movies/TV
2. Informal "sen" for casual dialogue, friends, conflicts
3. Formal "siz" only for official/hierarchical contexts
4. Adapt idioms to Turkish equivalents
5. Keep proper names unchanged (Neo, Oppenheimer, etc.)
6. Keep sentence length similar to source

### TECHNICAL RULES:
1. Return blocks in SAME <b id="N">...</b> tags
2. Preserve ALL HTML tags exactly: <font>, <b>, </b>, </font>
3. Keep tags in SAME positions
4. Multi-line dialogue MUST stay multi-line
5. OUTPUT: Only XML blocks, no explanations
6. Translate ALL blocks — if I send 500, return all 500

REMINDER: OUTPUT LANGUAGE = TURKISH (Türkçe) ONLY!

CRITICAL:
- If input has </b></font> at the end of dialogue, output MUST have </b></font> at the end!
- You MUST return EVERY SINGLE block I send to you. DO NOT skip any blocks!

### EXAMPLE 1 (Multi-line with tags):
Input:
<b id="0">
2
00:02:55,860 --> 00:02:59,450
<font size="20"><b>THREAT LEVEL: DEMON
SUPER MOUSE</b></font>
</b>

Output:
<b id="0">
2
00:02:55,860 --> 00:02:59,450
<font size="20"><b>TEHDİT SEVİYESİ: ŞEYTAN
SÜPER FARE</b></font>
</b>

Notice:
- 2 input lines → 2 output lines!
- <font size="20"><b> preserved at start
- </b></font> preserved at end!

### EXAMPLE 2 (Simple multi-line):
Input:
<b id="1">
17
00:02:03,557 --> 00:02:05,227
Well, I've only read
the transcripts.
</b>

Output:
<b id="1">
17
00:02:03,557 --> 00:02:05,227
Şey, ben sadece
transkriptleri okudum.
</b>`

// openingTagRe matches one or more opening HTML tags at the start of a line.
var openingTagRe = regexp.MustCompile(`^(<[^/][^>]*>)+`)

// closingTagRe matches one or more closing HTML tags at the end of a line.
var closingTagRe = regexp.MustCompile(`(</[^>]+>)+$`)

// lineFormatting holds the HTML tags stripped from a single dialogue line.
type lineFormatting struct {
	opening string
	closing string
	content string
}

// extractLineFormatting strips leading opening tags and trailing closing tags from a
// dialogue line, returning them separately so clean content can be sent to the model.
func extractLineFormatting(line string) lineFormatting {
	lf := lineFormatting{}

	if m := openingTagRe.FindString(line); m != "" {
		lf.opening = m
		line = line[len(m):]
	}
	if m := closingTagRe.FindString(line); m != "" {
		lf.closing = m
		line = line[:len(line)-len(m)]
	}
	lf.content = strings.TrimSpace(line)
	return lf
}

// restoreLineFormatting re-wraps a translated dialogue line with its original tags.
func restoreLineFormatting(translated string, lf lineFormatting) string {
	return lf.opening + translated + lf.closing
}

type GeminiTranslator struct {
	modelName string
}

func NewGeminiTranslator() *GeminiTranslator {
	return &GeminiTranslator{
		modelName: "gemini-2.5-flash-lite",
	}
}

func (g *GeminiTranslator) TranslateBatch(ctx context.Context, blocks []port.SRTBlock, keyValue string) ([]port.SRTBlock, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: keyValue})
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	// Per-block, per-line: strip leading/trailing HTML tags before sending.
	// Store formatting per block so it can be restored after translation.
	type blockMeta struct {
		lineFmts []lineFormatting
	}
	metas := make([]blockMeta, len(blocks))
	cleanBlocks := make([]port.SRTBlock, len(blocks))

	for i, b := range blocks {
		lines := strings.Split(b.Text, "\n")
		fmts := make([]lineFormatting, len(lines))
		cleanLines := make([]string, len(lines))
		for j, line := range lines {
			lf := extractLineFormatting(line)
			fmts[j] = lf
			cleanLines[j] = lf.content
		}
		metas[i] = blockMeta{lineFmts: fmts}
		cleanBlocks[i] = port.SRTBlock{
			Index:     b.Index,
			Timestamp: b.Timestamp,
			Text:      strings.Join(cleanLines, "\n"),
		}
	}

	// Build XML batch prompt
	var sb strings.Builder
	for idx, b := range cleanBlocks {
		sb.WriteString(fmt.Sprintf("<b id=\"%d\">\n%d\n%s\n%s\n</b>\n\n", idx, b.Index, b.Timestamp, b.Text))
	}

	promptContent := fmt.Sprintf(
		"CRITICAL: OUTPUT LANGUAGE MUST BE TURKISH (Türkçe)!\n\n"+
			"INSTRUCTIONS:\n"+
			"1. TRANSLATE ALL dialogue to TURKISH (Türkçe) — source can be any language\n"+
			"2. OUTPUT LANGUAGE: TURKISH (Türkçe) ONLY\n"+
			"3. PRESERVE subtitle numbers and timestamps EXACTLY\n"+
			"4. PRESERVE line breaks: 2 lines in source = 2 lines in output\n"+
			"5. DO NOT merge lines!\n\n"+
			"Translate these %d blocks to TURKISH (Türkçe):\n\n%s\n"+
			"REMINDER: All dialogue MUST be in TURKISH (Türkçe) language!",
		len(cleanBlocks), sb.String(),
	)

	temperature := float32(0.3)
	result, err := client.Models.GenerateContent(ctx, g.modelName, genai.Text(promptContent), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser),
		Temperature:       &temperature,
	})
	if err != nil {
		return nil, g.handleError(err)
	}

	translated, err := g.parseResponse(result.Text(), cleanBlocks)
	if err != nil {
		return nil, err
	}

	// Restore per-line HTML tags
	for i := range translated {
		if i >= len(metas) {
			break
		}
		lines := strings.Split(translated[i].Text, "\n")
		fmts := metas[i].lineFmts
		restored := make([]string, len(lines))
		for j, line := range lines {
			if j < len(fmts) {
				restored[j] = restoreLineFormatting(line, fmts[j])
			} else {
				restored[j] = line
			}
		}
		translated[i].Text = strings.Join(restored, "\n")
	}

	return translated, nil
}

func (g *GeminiTranslator) parseResponse(response string, original []port.SRTBlock) ([]port.SRTBlock, error) {
	var translated []port.SRTBlock

	parts := strings.Split(response, "</b>")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		start := strings.Index(part, ">")
		if start == -1 {
			continue
		}
		content := strings.TrimSpace(part[start+1:])
		lines := strings.SplitN(content, "\n", 3)
		if len(lines) < 3 {
			continue
		}
		var index int
		fmt.Sscanf(lines[0], "%d", &index)
		translated = append(translated, port.SRTBlock{
			Index:     index,
			Timestamp: lines[1],
			Text:      lines[2],
		})
	}

	if len(translated) != len(original) {
		return nil, fmt.Errorf("block count mismatch: got %d, want %d", len(translated), len(original))
	}

	return translated, nil
}

func (g *GeminiTranslator) handleError(err error) error {
	errStr := strings.ToLower(err.Error())
	is429 := strings.Contains(errStr, "429") || strings.Contains(errStr, "resource_exhausted")
	if !is429 {
		return err
	}
	if strings.Contains(errStr, "per minute") || strings.Contains(errStr, "per_minute") {
		return fmt.Errorf("quota_exhausted_rpm: %w", err)
	}
	if strings.Contains(errStr, "per day") || strings.Contains(errStr, "per_day") ||
		strings.Contains(errStr, "daily") {
		return fmt.Errorf("quota_exhausted_rpd: %w", err)
	}
	return fmt.Errorf("quota_exhausted: %w", err)
}
