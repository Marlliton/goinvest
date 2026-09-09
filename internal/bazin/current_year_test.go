package bazin_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCurrentYearConcentration_IgnoresPastYears(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2025", 1.00, cash),
		event("10/06/2026", 0.50, cash),
		event("10/09/2026", 0.50, cash),
	}

	got, ok := bazin.CurrentYearConcentration(events, now)
	require.True(t, ok)
	require.InDelta(t, 0.5, got, 1e-9)
}

func TestCurrentYearConcentration_SingleEventIsConcentrated(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/06/2026", 1.00, cash),
	}

	got, ok := bazin.CurrentYearConcentration(events, now)
	require.True(t, ok)
	require.Greater(t, got, 0.5)
}

func TestCurrentYearConcentration_NoEventsInCurrentYear(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/06/2025", 1.00, cash),
	}

	_, ok := bazin.CurrentYearConcentration(events, now)
	require.False(t, ok)
}

func TestCurrentYearConcentration_IgnoresInvalidSharesFactor(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/06/2026", 1.00, cash),
		event("11/06/2026", 5.00, cash),
	}
	events[1].SharesFactor = 0

	got, ok := bazin.CurrentYearConcentration(events, now)
	require.True(t, ok)
	require.InDelta(t, 1.0, got, 1e-9)
}
