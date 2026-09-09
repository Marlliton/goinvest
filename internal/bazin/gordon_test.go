package bazin_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/stretchr/testify/require"
)

func TestRequiredReturn(t *testing.T) {
	require.InDelta(t, 0.20, bazin.RequiredReturn(0.14), 1e-9)
}

func TestGordon_Ceiling(t *testing.T) {
	got, ok := bazin.Gordon(2.0, 0.20, 0.05)
	require.True(t, ok)
	require.InDelta(t, 2.0/0.15, got.Ceiling, 1e-9)
	require.InDelta(t, 0.20, got.Ke, 1e-9)
	require.InDelta(t, 0.05, got.G, 1e-9)
}

func TestGordon_GrowthEqualsRequiredReturn(t *testing.T) {
	_, ok := bazin.Gordon(2.0, 0.20, 0.20)
	require.False(t, ok)
}

func TestGordon_GrowthAboveRequiredReturn(t *testing.T) {
	_, ok := bazin.Gordon(2.0, 0.20, 0.25)
	require.False(t, ok)
}
