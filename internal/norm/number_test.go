package norm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

func TestParseBRNumber(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  float64
		ok    bool
	}{
		{"plain decimal", "49,70", 49.70, true},
		{"thousands separator without decimal", "208.607.000.000", 208607000000, true},
		{"thousands separator with decimal", "1.012.240.000,00", 1012240000.00, true},
		{"negative", "-0,20", -0.20, true},
		{"dash means absent not zero", "-", 0, false},
		{"empty string means absent", "", 0, false},
		{"NA marker means absent", "N/A", 0, false},
		{"parse error never yields a valid zero", "12,3,4", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := norm.ParseBRNumber(c.input)
			require.Equal(t, c.ok, ok, "presence flag for %q", c.input)
			require.InDelta(t, c.want, got, 1e-9, "value for %q", c.input)
		})
	}
}

func TestParseBRPercent(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  float64
		ok    bool
	}{
		{"positive percent becomes a fraction", "33,1%", 0.331, true},
		{"large negative percent", "-362,66%", -3.6266, true},
		{"explicit zero is valid in the generic parser", "0,0%", 0, true},
		{"absence stays absence", "-", 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := norm.ParseBRPercent(c.input)
			require.Equal(t, c.ok, ok, "presence flag for %q", c.input)
			require.InDelta(t, c.want, got, 1e-9, "value for %q", c.input)
		})
	}
}
