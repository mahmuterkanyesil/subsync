package srt

import (
	"strings"
	"subsync/internal/core/application/port"
)

func IsTurkish(blocks []port.SRTBlock) bool {
	turkishChars := []string{"ğ", "ı", "ş", "ç", "ö", "ü", "Ğ", "İ", "Ş"}
	englishMarkers := []string{" the ", " and ", " is ", " are ", " was "}

	sample := ""
	for i, b := range blocks {
		if i < 5 || i == len(blocks)/2 {
			sample += b.Text + " "
		}
	}
	sample = strings.ToLower(sample)

	for _, m := range englishMarkers {
		if strings.Contains(sample, m) {
			return false
		}
	}

	for _, c := range turkishChars {
		if strings.Contains(sample, c) {
			return true
		}
	}

	return false
}
