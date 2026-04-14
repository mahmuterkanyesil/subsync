package domainservice_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	domainservice "subsync/internal/core/domain/service"
)

func TestIsTranslatedToTurkish(t *testing.T) {
	tests := []struct {
		name   string
		inputs []string
		want   bool
	}{
		// Turkish characters → true
		{"contains ğ", []string{"Bu gerçekten ğüzel bir gün"}, true},
		{"contains ı", []string{"Işıklı bir sabah"}, true},
		{"contains ş", []string{"Şimdi buradayım"}, true},
		{"contains ç", []string{"Çok güzel bir yer"}, true},
		{"contains ö", []string{"Öğretmen bana baktı"}, true},
		{"contains ü", []string{"Üzgün bir çocuk"}, true},
		{"contains capital Ğ", []string{"DEĞER"}, true},
		{"contains capital İ", []string{"İstanbul çok güzel"}, true},
		{"contains capital Ş", []string{"Şeker yedim"}, true},
		{"multi-block joined", []string{"Bugün", "ğüzel", "bir gün"}, true},

		// English markers → false
		{"english marker ' the '", []string{"He went to the store"}, false},
		{"english marker ' and '", []string{"cats and dogs are great"}, false},
		{"english marker ' is '", []string{"She is very happy"}, false},
		{"english marker ' are '", []string{"They are coming"}, false},
		{"english marker ' was '", []string{"It was a dark night"}, false},

		// French markers → false
		{"french marker ' le '", []string{"Le chat est noir"}, false},
		{"french marker ' la '", []string{"la maison est belle"}, false},
		{"french marker ' les '", []string{"les enfants jouent"}, false},

		// Spanish markers → false
		{"spanish marker ' el '", []string{"el gato está aquí"}, false},
		{"spanish marker ' los '", []string{"los niños juegan"}, false},

		// Non-Latin scripts → false
		{"cyrillic script", []string{"Привет мир как дела"}, false},
		{"arabic script", []string{"مرحبا بالعالم"}, false},
		{"cjk script", []string{"你好世界"}, false},

		// Edge cases
		{"empty slice", []string{}, false},
		{"single empty string", []string{""}, false},
		{"no turkish chars no markers", []string{"Hello world"}, false},
		// English marker (surrounded by spaces) wins over Turkish chars
		{"turkish chars + english marker mid-sentence", []string{"went to the ğüzel market"}, false},
		// No markers but also no Turkish chars
		{"latin text no markers no turkish", []string{"Greetings from far away"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domainservice.IsTranslatedToTurkish(tt.inputs)
			assert.Equal(t, tt.want, got)
		})
	}
}
