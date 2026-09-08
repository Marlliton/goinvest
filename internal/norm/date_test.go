package norm_test

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/norm"
	"github.com/stretchr/testify/require"
)

func TestParseBRDate(t *testing.T) {
	cases := map[string]struct {
		want time.Time
		ok   bool
	}{
		"14/08/2003":  {time.Date(2003, time.August, 14, 0, 0, 0, 0, time.UTC), true},
		" 30/06/2026": {time.Date(2026, time.June, 30, 0, 0, 0, 0, time.UTC), true},
		"-":           {},
		"":            {},
		"N/A":         {},
		"2003-08-14":  {},
		"32/01/2026":  {},
	}

	for in, want := range cases {
		got, ok := norm.ParseBRDate(in)
		require.Equal(t, want.ok, ok, "entrada %q", in)
		require.Equal(t, want.want, got, "entrada %q", in)
	}
}
