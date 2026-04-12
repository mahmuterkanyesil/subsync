package domainservice

import "strings"

// IsTranslatedToTurkish, verilen metin bloklarının Türkçe olup olmadığını doğrular.
// Başka dil marker'larına veya script'lerine ait içerik tespit edilirse false döner.
func IsTranslatedToTurkish(texts []string) bool {
	turkishChars := []string{"ğ", "ı", "ş", "ç", "ö", "ü", "Ğ", "İ", "Ş"}
	englishMarkers := []string{" the ", " and ", " is ", " are ", " was "}
	frenchMarkers := []string{" le ", " la ", " les ", " est ", " sont "}
	spanishMarkers := []string{" el ", " los ", " es ", " son "}

	sample := strings.Join(texts, " ")
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
