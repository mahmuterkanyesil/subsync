package srt

import (
	domainservice "subsync/internal/core/domain/service"
	"subsync/internal/core/domain/valueobject"
)

// IsTurkish, verilen SRT bloklarının Türkçe çeviri içerip içermediğini doğrular.
// Domain service'e delege eder.
func IsTurkish(blocks []valueobject.SRTBlock) bool {
	texts := make([]string, len(blocks))
	for i, b := range blocks {
		texts[i] = b.Text
	}
	return domainservice.IsTranslatedToTurkish(texts)
}
