package bazin_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/stretchr/testify/require"
)

func TestMedianDY_FiveYears(t *testing.T) {
	result := bazin.Result{Years: []bazin.YearlyDividend{
		{Year: 2025, Total: 5.0},
		{Year: 2024, Total: 4.0},
		{Year: 2023, Total: 3.0},
		{Year: 2022, Total: 2.0},
		{Year: 2021, Total: 1.0},
	}}

	got, ok := bazin.MedianDividendYield(result, 100.0)
	require.True(t, ok)
	require.InDelta(t, 0.03, got, 1e-9)
}

func TestMedianDY_EvenYears(t *testing.T) {
	result := bazin.Result{Years: []bazin.YearlyDividend{
		{Year: 2025, Total: 4.0},
		{Year: 2024, Total: 3.0},
		{Year: 2023, Total: 2.0},
		{Year: 2022, Total: 1.0},
	}}

	got, ok := bazin.MedianDividendYield(result, 100.0)
	require.True(t, ok)
	require.InDelta(t, 0.025, got, 1e-9)
}

func TestMedianDY_NonPositivePrice(t *testing.T) {
	result := bazin.Result{Years: []bazin.YearlyDividend{{Year: 2025, Total: 1.0}}}

	_, ok := bazin.MedianDividendYield(result, 0)
	require.False(t, ok)
}

func TestMedianDY_NoYears(t *testing.T) {
	_, ok := bazin.MedianDividendYield(bazin.Result{}, 100.0)
	require.False(t, ok)
}
