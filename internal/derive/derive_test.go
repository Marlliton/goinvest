package derive_test

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func ptr(v float64) *float64 { return &v }

var collectedAt = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

// Percentuais ficam na escala que a fonte publica (25.0 é 25%), então payout e
// roa saem também em ponto percentual.
func healthySet() domain.MetricSet {
	values := map[domain.MetricID]*float64{
		"dy":         ptr(1.2),
		"pl":         ptr(30.0),
		"pvp":        ptr(9.5),
		"patrim_liq": ptr(15e9),
		"dl_patrim":  ptr(0.10),
		"ev_ebitda":  ptr(20.0),
		"roe":        ptr(25.0),
		"p_ativo":    ptr(4.0),
	}
	set := domain.MetricSet{}
	for id, v := range values {
		set[id] = domain.Observation{
			Ticker: "WEGE3", Metric: id, PeriodKind: "spot",
			Value: v, Source: "fundamentus:resultado", FetchedAt: collectedAt,
		}
	}
	return set
}

func TestComputeProducesAllThreeDerivedMetrics(t *testing.T) {
	out := derive.Compute(healthySet())
	require.Len(t, out, 3)

	// payout = dy * pl
	require.InDelta(t, 1.2*30.0, *out["payout"].Value, 1e-9)

	// MC = 9,5 x 15e9 = 142,5e9 | DL = 0,10 x 15e9 = 1,5e9
	// EV = 144e9 | EBITDA = 144e9 / 20 = 7,2e9 | DL/EBITDA = 1,5 / 7,2
	require.InDelta(t, 1.5e9/7.2e9, *out["dl_ebitda"].Value, 1e-9)

	// AtivoTotal = 142,5e9 / 4 = 35,625e9 | Lucro = 25 x 15e9 = 375e9
	require.InDelta(t, 375e9/35.625e9, *out["roa"].Value, 1e-9)
}

// A cadeia longa existe para localizar o insumo ruim, mas precisa concordar
// com a redução algébrica: se divergir, a aritmética está errada.
func TestChainAgreesWithAlgebraicReduction(t *testing.T) {
	out := derive.Compute(healthySet())
	require.InDelta(t, 25.0*4.0/9.5, *out["roa"].Value, 1e-9, "ROA reduz para roe x p_ativo / pvp")
	require.InDelta(t, 0.10*20.0/(9.5+0.10), *out["dl_ebitda"].Value, 1e-9,
		"DL/EBITDA reduz para dl_patrim x ev_ebitda / (pvp + dl_patrim)")
}

func TestComputeCarriesProvenanceOfInputs(t *testing.T) {
	out := derive.Compute(healthySet())
	require.Equal(t, "derive:payout", out["payout"].Source)
	require.Equal(t, domain.UnitPercent, out["payout"].Unit)
	require.Equal(t, domain.UnitRatio, out["dl_ebitda"].Unit)
	require.Equal(t, "WEGE3", out["roa"].Ticker)
	require.Equal(t, collectedAt, out["roa"].FetchedAt)
}

func TestDerivedInheritsOldestInputFreshness(t *testing.T) {
	set := healthySet()
	stale := set["p_ativo"]
	stale.FetchedAt = collectedAt.Add(-72 * time.Hour)
	set["p_ativo"] = stale

	out := derive.Compute(set)
	require.Equal(t, stale.FetchedAt, out["roa"].FetchedAt,
		"derivado não é mais fresco que o pior insumo")
	require.Equal(t, collectedAt, out["payout"].FetchedAt, "payout não usa p_ativo")
}

func TestSuspectInputRemovesTheKeyEntirely(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(domain.MetricSet)
		missing domain.MetricID
	}{
		{
			name:    "ev_ebitda ausente",
			mutate:  func(m domain.MetricSet) { o := m["ev_ebitda"]; o.Value = nil; m["ev_ebitda"] = o },
			missing: "dl_ebitda",
		},
		{
			name:    "ev_ebitda no sentinela zero, como vem para banco",
			mutate:  func(m domain.MetricSet) { o := m["ev_ebitda"]; o.Value = ptr(0); m["ev_ebitda"] = o },
			missing: "dl_ebitda",
		},
		{
			name:    "pl nunca coletado",
			mutate:  func(m domain.MetricSet) { delete(m, "pl") },
			missing: "payout",
		},
		{
			name:    "p_ativo zero levaria o ativo total a infinito",
			mutate:  func(m domain.MetricSet) { o := m["p_ativo"]; o.Value = ptr(0); m["p_ativo"] = o },
			missing: "roa",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			set := healthySet()
			tc.mutate(set)
			out := derive.Compute(set)

			_, present := out[tc.missing]
			require.False(t, present, "a ausência é da chave, não um valor nulo dentro dela")
			require.Len(t, out, 2, "os outros dois derivados continuam saindo")
		})
	}
}

func TestComputeOnEmptySetProducesNothing(t *testing.T) {
	out := derive.Compute(domain.MetricSet{})
	require.NotNil(t, out)
	require.Empty(t, out)
}
