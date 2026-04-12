package gemini

import (
	"context"
	"fmt"
	"strings"
	"subsync/internal/core/application/port"

	"google.golang.org/genai"
)

const systemInstruction = `Sen bir altyazı çevirmenisin.
Sana verilen altyazı bloklarını Türkçe'ye çevir.
SADECE Türkçe çıktı ver.
Altyazı numaralarını ve zaman damgalarını AYNEN koru.
HTML taglerini AYNEN koru.
Satır sayısını koru — 2 satır geldiyse 2 satır döndür.`

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

func (g *GeminiTranslator) TranslateBatch(ctx context.Context, blocks []port.SRTBlock, keyValue string) ([]port.SRTBlock, error) {
	// Blokları XML formatına çevir
	var sb strings.Builder
	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("<b id=\"%d\">\n%d\n%s\n%s\n</b>\n", b.Index, b.Index, b.Timestamp, b.Text))
	}

	model := g.client.Models
	result, err := model.GenerateContent(ctx, g.modelName, genai.Text(sb.String()), &genai.GenerateContentConfig{
		SystemInstruction: genai.NewContentFromText(systemInstruction, genai.RoleUser),
	})
	if err != nil {
		return nil, g.handleError(err)
	}

	// Yanıtı parse et
	response := result.Text()
	return g.parseResponse(response, blocks)
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
	errStr := err.Error()
	if strings.Contains(errStr, "429") || strings.Contains(errStr, "resource_exhausted") {
		return fmt.Errorf("quota_exhausted: %w", err)
	}
	return err
}
