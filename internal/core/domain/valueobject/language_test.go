package valueobject_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"subsync/internal/core/domain/valueobject"
)

func TestLookupLanguage_Known(t *testing.T) {
	spec, ok := valueobject.LookupLanguage("tr")
	require.True(t, ok)
	assert.Equal(t, "tr", spec.Code)
	assert.Equal(t, "tur", spec.FFmpegCode)
	assert.Equal(t, "Turkish", spec.NameEN)
	assert.Equal(t, "Türkçe", spec.NameNative)
}

func TestLookupLanguage_Spanish(t *testing.T) {
	spec, ok := valueobject.LookupLanguage("es")
	require.True(t, ok)
	assert.Equal(t, "es", spec.Code)
	assert.Equal(t, "spa", spec.FFmpegCode)
}

func TestLookupLanguage_Unknown(t *testing.T) {
	_, ok := valueobject.LookupLanguage("xx")
	assert.False(t, ok)
}

func TestDefaultLanguage(t *testing.T) {
	lang := valueobject.DefaultLanguage()
	assert.Equal(t, "tr", lang.Code)
}

func TestSupportedLanguages_AllHaveRequiredFields(t *testing.T) {
	for code, spec := range valueobject.SupportedLanguages {
		assert.Equal(t, code, spec.Code, "code mismatch for %s", code)
		assert.NotEmpty(t, spec.FFmpegCode, "missing FFmpegCode for %s", code)
		assert.NotEmpty(t, spec.NameEN, "missing NameEN for %s", code)
		assert.NotEmpty(t, spec.NameNative, "missing NameNative for %s", code)
	}
}
