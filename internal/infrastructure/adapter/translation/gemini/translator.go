package gemini

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"subsync/internal/core/application/port"

	"google.golang.org/genai"
)

const systemInstruction = `Sen bir altyazı çevirmenisin.
Sana verilen altyazı bloklarını Türkçe'ye çevir.
SADECE Türkçe çıktı ver.
Altyazı numaralarını ve zaman damgalarını AYNEN koru.
HTML taglerini AYNEN koru.
Satır sayısını koru — 2 satır geldiyse 2 satır döndür.
Diyaloglarda "sen" kullan, resmi "siz" yerine.
Türkçe noktalama kurallarına uy.`

var htmlTagRe = regexp.MustCompile(`<[^>]+>`)

type GeminiTranslator struct {
	client    *genai.Client
	modelName string
	rpmLimit  int
	rpdLimit  int
	tpmLimit  int
}

func NewGeminiTranslator(ctx context.Context) (*GeminiTranslator, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{})
	if err != nil {
		return nil, err
	}

	return &GeminiTranslator{
		client:    client,
		modelName: "gemini-2.5-flash-lite",
		rpmLimit:  10,
		rpdLimit:  20,
		tpmLimit:  250000,
	}, nil
}

func stripTags(text string) (string, []string) {
	var tags []string
	stripped := htmlTagRe.ReplaceAllStringFunc(text, func(tag string) string {
		i := len(tags)
		tags = append(tags, tag)
		return fmt.Sprintf("[T%d]", i)
	})
	return stripped, tags
}

func restoreTags(text string, tags []string) string {
	for i, tag := range tags {
		text = strings.ReplaceAll(text, fmt.Sprintf("[T%d]", i), tag)
	}
	return text
}

func (g *GeminiTranslator) TranslateBatch(ctx context.Context, blocks []port.SRTBlock, keyValue string) ([]port.SRTBlock, error) {
	// Strip HTML tags before sending, store per block
	strippedBlocks := make([]port.SRTBlock, len(blocks))
	blockTags := make([][]string, len(blocks))
	for i, b := range blocks {
		stripped, tags := stripTags(b.Text)
		strippedBlocks[i] = port.SRTBlock{
			Index:     b.Index,
			Timestamp: b.Timestamp,
			Text:      stripped,
		}
		blockTags[i] = tags
	}

	// Build XML batch
	var sb strings.Builder
	for _, b := range strippedBlocks {
		sb.WriteString(fmt.Sprintf("<b id=\"%d\">\n%d\n%s\n%s\n</b>\n", b.Index, b.Index, b.Timestamp, b.Text))
	}

	model := g.client.Models
	result, err := model.GenerateContent(ctx, g.modelName, genai.Text(sb.String()), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser),
	})
	if err != nil {
		return nil, g.handleError(err)
	}

	// Parse response
	response := result.Text()
	translated, err := g.parseResponse(response, strippedBlocks)
	if err != nil {
		return nil, err
	}

	// Restore HTML tags using batch position (0-based)
	for i := range translated {
		if i < len(blockTags) && len(blockTags[i]) > 0 {
			translated[i].Text = restoreTags(translated[i].Text, blockTags[i])
		}
	}

	return translated, nil
}

func (g *GeminiTranslator) parseResponse(response string, original []port.SRTBlock) ([]port.SRTBlock, error) {
	translated := []port.SRTBlock{}

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
