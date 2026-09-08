package app_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func alertOf(t *testing.T, r app.Report, id string) evaluate.Finding {
	t.Helper()
	for _, f := range r.Alerts {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("alerta %s ausente do relatório", id)
	return evaluate.Finding{}
}

// payout = DY × P/L: 0,45 × 2,5 = 112,5%.
func overDistributingValues() map[domain.MetricID]*float64 {
	values := wege3Values()
	values["dy"] = ptr(0.45)
	values["pl"] = ptr(2.5)
	return values
}

func TestShow_AlertBlock_Fires(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, overDistributingValues())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)

	fired := alertOf(t, report, "ALERTA-01")
	require.Equal(t, evaluate.StatusFired, fired.Status)
	require.InDelta(t, 1.125, fired.Numbers["payout"], 1e-9)

	text := app.RenderText(report)
	require.Contains(t, text, "Alertas")
	require.Contains(t, text, "ALERTA-01")
	require.Contains(t, text, fired.Rule)
	require.Contains(t, text, "112,50%", "o número que produziu o veredito fica visível")
	require.Contains(t, lineOf(t, report, "payout").AlertMarks, "ALERTA-01")
	require.Contains(t, lineWith(t, splitLines(text), "Payout"), "⚠")
}

func TestShow_AlertBlock_NoFiredAlerts(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.NotContains(t, text, "Alertas", "ativo saudável não ganha bloco vazio")
	require.NotContains(t, text, "⚠ ", "nenhuma linha marcada")
	require.Empty(t, lineOf(t, report, "payout").AlertMarks)
}

func TestShow_Alerts_AlwaysPopulated(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87), "pvp": ptr(1.02), "dy": ptr(0.132),
	})

	for _, ticker := range []string{"WEGE3", "MXRF11"} {
		report, err := app.Show(t.Context(), db, loadCatalog(t), ticker, now)
		require.NoError(t, err)
		require.Len(t, report.Alerts, 5, ticker)
		for _, f := range report.Alerts {
			require.NotEmpty(t, f.Status, "%s/%s", ticker, f.ID)
		}
	}
}

func TestShow_AlertBlock_WithBazin(t *testing.T) {
	db := openTemp(t)
	values := overDistributingValues()
	values["cotacao"] = ptr(24.00)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "BBAS3", now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "Preço-teto (Bazin)")
	require.Contains(t, text, "Teto: R$ 20,00")
	require.Contains(t, text, "ALERTA-01", "o teto e o alerta de payout nunca saem em telas separadas")
}

// Em FII o segmento chega em Sector: ler asset.Segment deixa o alerta sempre não aplicável.
func TestShow_AlertVacancyUsesFIITaxonomyLabel(t *testing.T) {
	db := openTemp(t)
	// MinPeerGroup exige cinco papéis líquidos para haver percentil.
	seedFIIPeers(t, db, "Logística", map[string]float64{
		"AAAA11": 0.05, "BBBB11": 0.07, "CCCC11": 0.08,
		"DDDD11": 0.09, "EEEE11": 0.10, "MXRF11": 0.14,
	})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "MXRF11", now)
	require.NoError(t, err)

	got := alertOf(t, report, "ALERTA-05")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 0.12, got.Numbers["limiar"], 1e-9)
	require.Contains(t, app.RenderText(report), "ALERTA-05")
}

func seedFIIPeers(t *testing.T, db *store.DB, sector string, dyByTicker map[string]float64) {
	t.Helper()
	for ticker, dy := range dyByTicker {
		values := map[domain.MetricID]*float64{
			"cotacao": ptr(9.87), "pvp": ptr(1.02), "dy": ptr(dy),
			"liquidez_fii": ptr(4_500_000), "vacancia_media": ptr(0.15),
		}
		seed(t, db, ticker, domain.ClassFII, values)
		a, found, err := db.GetAsset(t.Context(), ticker)
		require.NoError(t, err)
		require.True(t, found)
		require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, true, collectedAt))
		setIdentity(t, db, ticker, sector, "", "")
	}
	require.NoError(t, db.RecomputeSectorStats(t.Context(),
		[]store.MetricRule{{MetricID: "dy"}}, collectedAt))
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return out
}
