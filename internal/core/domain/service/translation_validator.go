package domainservice

import "strings"

var (
	englishMarkers = []string{" the ", " and ", " is ", " are ", " was ", " were ", " have ", " has ", " with ", " from ", " that ", " this "}
	frenchMarkers  = []string{" le ", " la ", " les ", " un ", " une ", " des ", " est ", " sont ", " dans ", " pour ", " avec ", " pas "}
	spanishMarkers = []string{" el ", " los ", " las ", " un ", " una ", " es ", " son ", " en ", " para ", " con "}

	// ğ, ı, ş are Turkish-unique — not present in French/Spanish/German
	turkishUnique = []string{"ğ", "ı", "ş", "Ğ", "İ", "Ş"}
	// ç, ö, ü also exist in French and German — only accept when no other-language markers found
	turkishCommon = []string{"ç", "ö", "ü", "Ç", "Ö", "Ü"}
)

// IsTranslatedToTurkish validates that the given subtitle text blocks are in Turkish.
// Samples strategically from the beginning, quarter, middle, three-quarter, and end of
// the block list to catch cases where only part of the translation is in Turkish.
func IsTranslatedToTurkish(texts []string) bool {
	if len(texts) == 0 {
		return false
	}

	// Strategic sampling: beginning, 25%, middle, 75%, end
	indices := []int{0, len(texts) / 4, len(texts) / 2, 3 * len(texts) / 4, len(texts) - 1}
	seen := map[int]bool{}
	var sampleParts []string
	for _, i := range indices {
		if i >= 0 && i < len(texts) && !seen[i] {
			sampleParts = append(sampleParts, texts[i])
			seen[i] = true
		}
	}
	sample := strings.Join(sampleParts, " ")
	sampleLower := strings.ToLower(sample)

	// Reject other-language markers first
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

	// Reject non-Latin scripts
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

	// Turkish-unique characters confirm Turkish unambiguously
	for _, c := range turkishUnique {
		if strings.Contains(sample, c) {
			return true
		}
	}

	// Turkish-common characters are sufficient when no other-language markers were found
	for _, c := range turkishCommon {
		if strings.Contains(sample, c) {
			return true
		}
	}

	return false
}
