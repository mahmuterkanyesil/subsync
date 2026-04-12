package srt

import (
	"strings"
	"subsync/internal/core/application/port"
)

func IsTurkish(blocks []port.SRTBlock) bool {
	turkishChars := []string{"ğ", "ı", "ş", "ç", "ö", "ü", "Ğ", "İ", "Ş"}
	englishMarkers := []string{" the ", " and ", " is ", " are ", " was "}
	frenchMarkers := []string{" le ", " la ", " les ", " est ", " sont "}
	spanishMarkers := []string{" el ", " los ", " es ", " son "}

	sample := ""
	for i, b := range blocks {
		if i < 5 || i == len(blocks)/2 {
			sample += b.Text + " "
		}
	}
	sampleLower := strings.ToLower(sample)

	for _, m := range englishMarkers {
		if strings.Contains(sampleLower, m) {
			return false
		}
	}
	for _, m := range frenchMarkers {
		if strings.Contains(sampleLower, m) {
			return false
		}
	}
	for _, m := range spanishMarkers {
		if strings.Contains(sampleLower, m) {
			return false
		}
	}

	for _, r := range sample {
		switch {
		case r >= '\u0400' && r <= '\u04FF':
			return false // Cyrillic
		case r >= '\u0600' && r <= '\u06FF':
			return false // Arabic
		case r >= '\u4E00' && r <= '\u9FFF':
			return false // CJK
		}
	}

	for _, c := range turkishChars {
		if strings.Contains(sample, c) {
			return true
		}
	}

	return false
}
