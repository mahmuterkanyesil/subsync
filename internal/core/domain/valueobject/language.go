package valueobject

type LanguageSpec struct {
	Code       string // ISO 639-1: "tr", "es", "fr"
	FFmpegCode string // ISO 639-2/B: "tur", "spa", "fra"
	NameEN     string // "Turkish"
	NameNative string // "Türkçe"
}

var SupportedLanguages = map[string]LanguageSpec{
	"tr": {"tr", "tur", "Turkish", "Türkçe"},
	"es": {"es", "spa", "Spanish", "Español"},
	"fr": {"fr", "fra", "French", "Français"},
	"de": {"de", "ger", "German", "Deutsch"},
	"it": {"it", "ita", "Italian", "Italiano"},
	"pt": {"pt", "por", "Portuguese", "Português"},
	"ru": {"ru", "rus", "Russian", "Русский"},
	"ar": {"ar", "ara", "Arabic", "العربية"},
	"ja": {"ja", "jpn", "Japanese", "日本語"},
	"ko": {"ko", "kor", "Korean", "한국어"},
	"zh": {"zh", "chi", "Chinese", "中文"},
	"nl": {"nl", "dut", "Dutch", "Nederlands"},
	"pl": {"pl", "pol", "Polish", "Polski"},
}

func LookupLanguage(code string) (LanguageSpec, bool) {
	spec, ok := SupportedLanguages[code]
	return spec, ok
}

func DefaultLanguage() LanguageSpec {
	return SupportedLanguages["tr"]
}
