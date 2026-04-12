package srt

import (
	"fmt"
	"strings"
	"subsync/internal/core/application/port"
)

func Parse(content string) []port.SRTBlock {
	blocks := []port.SRTBlock{}
	parts := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n\n")

	for _, part := range parts {
		lines := strings.Split(strings.TrimSpace(part), "\n")
		if len(lines) < 3 {
			continue
		}

		index := 0
		fmt.Sscanf(lines[0], "%d", &index)

		blocks = append(blocks, port.SRTBlock{
			Index:     index,
			Timestamp: lines[1],
			Text:      strings.Join(lines[2:], "\n"),
		})
	}

	return blocks
}

func Format(blocks []port.SRTBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("%d\n%s\n%s\n\n", b.Index, b.Timestamp, b.Text))
	}
	return sb.String()
}
