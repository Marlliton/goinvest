package derive_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func decompositionSet(pl, dy, roe, payout *float64) domain.MetricSet {
	values := map[domain.MetricID]*float64{
		"pl": pl, "dy": dy, "roe": roe, "payout": payout,
	}
	set := domain.MetricSet{}
	for id, v := range values {
		if v == nil {
			continue
		}
		set[id] = domain.Observation{
			Ticker: "WEGE3", Metric: id, PeriodKind: "spot",
			Value: v, Source: "fundamentus:resultado", FetchedAt: collectedAt,
		}
	}
	return set
}

func TestDecomposeHealthyCase(t *testing.T) {
	set := decompositionSet(ptr(15), ptr(0.03), ptr(0.20), ptr(0.45))

	got, ok := derive.Decompose(set)
	require.True(t, ok)
	require.InDelta(t, 1.0/15.0, got.EarningsYield, 1e-9)
	require.InDelta(t, 0.03, got.Distributed, 1e-9)
	require.InDelta(t, 0.55, got.Retained, 1e-9)
	require.InDelta(t, 0.11, got.ImpliedGrowth, 1e-9)
	require.InDelta(t, 0.14, got.TotalReturn, 1e-9)
	require.InDelta(t, 15.0, got.PaybackYears, 1e-9)
}

func TestDecomposeNotApplicable(t *testing.T) {
	cases := []struct {
		name                string
		pl, dy, roe, payout *float64
	}{
		{"pl ausente", nil, ptr(0.03), ptr(0.20), ptr(0.45)},
		{"pl negativo (prejuízo)", ptr(-3), ptr(0.03), ptr(0.20), ptr(0.45)},
		{"roe ausente", ptr(15), ptr(0.03), nil, ptr(0.45)},
		{"payout ausente", ptr(15), ptr(0.03), ptr(0.20), nil},
		{"payout acima de 100%", ptr(15), ptr(0.09), ptr(0.20), ptr(1.35)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := decompositionSet(tc.pl, tc.dy, tc.roe, tc.payout)

			got, ok := derive.Decompose(set)
			require.False(t, ok)
			require.Equal(t, derive.Decomposition{}, got)
		})
	}
}
