package norm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

func TestIsAbsenceSentinel(t *testing.T) {
	require.True(t, norm.IsAbsenceSentinel("ev_ebitda", 0),
		"EV/EBITDA of 0 is an absence sentinel, not a real value")

	require.False(t, norm.IsAbsenceSentinel("ev_ebitda", 4.2))

	require.False(t, norm.IsAbsenceSentinel("pl", 0),
		"a zero P/L is not a sentinel")
}
