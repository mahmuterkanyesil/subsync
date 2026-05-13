package gin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"normal unix path", "/media/movies/film.eng.srt", false},
		{"normal windows path", `C:\media\film.eng.srt`, false},
		{"traversal with ..", "/media/../etc/passwd", true},
		{"double traversal", "../../etc/passwd", true},
		{"traversal in middle", "/media/movies/../../etc/passwd", true},
		{"just ..", "..", true},
		{"encoded traversal after clean", "/media/./film.eng.srt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sanitizePath(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, got)
			}
		})
	}
}
