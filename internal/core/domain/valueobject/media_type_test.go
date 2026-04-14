package valueobject_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"subsync/internal/core/domain/valueobject"
)

func TestMediaType_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		input valueobject.MediaType
		want  bool
	}{
		{"series is valid", valueobject.MediaTypeSeries, true},
		{"movie is valid", valueobject.MediaTypeMovie, true},
		{"empty string invalid", valueobject.MediaType(""), false},
		{"arbitrary string invalid", valueobject.MediaType("documentary"), false},
		{"uppercase invalid", valueobject.MediaType("SERIES"), false},
		{"mixed case invalid", valueobject.MediaType("Movie"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.input.IsValid())
		})
	}
}
