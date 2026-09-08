package app_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func tableOf(t *testing.T, r app.CompareReport, class domain.AssetClass) app.CompareTable {
	t.Helper()
	for _, tb := range r.Tables {
		if tb.Class == class {
			return tb
		}
	}
	t.Fatalf("tabela da classe %s ausente", class)
	return app.CompareTable{}
}

func columnOf(t *testing.T, table app.CompareTable, ticker string) app.CompareColumn {
	t.Helper()
	for _, c := range table.Columns {
		if c.Ticker == ticker {
			return c
		}
	}
	t.Fatalf("coluna %s ausente", ticker)
	return app.CompareColumn{}
}

func seedStocks(t *testing.T, db *store.DB, tickers ...string) {
	t.Helper()
	for _, ticker := range tickers {
		seed(t, db, ticker, domain.ClassStock, wege3Values())
	}
}

func TestCompare_ThreeStocks(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "KEPL3"}, now)
	require.NoError(t, err)

	require.Len(t, report.Tables, 1)
	table := tableOf(t, report, domain.ClassStock)
	require.Len(t, table.Columns, 3)
	require.Equal(t, []string{"WEGE3", "ROMI3", "KEPL3"},
		[]string{table.Columns[0].Ticker, table.Columns[1].Ticker, table.Columns[2].Ticker},
		"a ordem digitada é a ordem das colunas")
	require.NotEmpty(t, table.Metrics)
	require.Empty(t, report.Invalid)

	wege := columnOf(t, table, "WEGE3")
	require.NotNil(t, wege.Cells["pl"].Value)
	require.InDelta(t, 30.0, *wege.Cells["pl"].Value, 1e-9)
}

func TestCompare_MixedClasses(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3")
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87), "pvp": ptr(1.02), "dy": ptr(0.132),
	})

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "MXRF11"}, now)
	require.NoError(t, err)

	require.Len(t, report.Tables, 2)
	stocks := tableOf(t, report, domain.ClassStock)
	fiis := tableOf(t, report, domain.ClassFII)
	require.Len(t, stocks.Columns, 2)
	require.Len(t, fiis.Columns, 1)

	require.False(t, hasMetric(stocks.Metrics, "vacancia_media"), "métrica de FII fora da tabela de ação")
	require.False(t, hasMetric(fiis.Metrics, "pl"), "métrica de ação fora da tabela de FII")
}

func TestCompare_InvalidTickerDoesNotAbort(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "WEGE33333", "ROMI3", "PETR4", "KEPL3"}, now)
	require.NoError(t, err)

	require.Len(t, tableOf(t, report, domain.ClassStock).Columns, 3)
	require.Len(t, report.Invalid, 2)

	byTicker := map[string]string{}
	for _, s := range report.Invalid {
		byTicker[s.Ticker] = s.Reason
	}
	require.Contains(t, byTicker, "WEGE33333")
	require.Contains(t, byTicker, "PETR4")
	require.NotEqual(t, byTicker["WEGE33333"], byTicker["PETR4"],
		"formato inválido e nunca sincronizado pedem ações opostas")
}

func TestCompare_DetailResolverAllOrNothing(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")
	seedDetail(t, db, "WEGE3", map[domain.MetricID]*float64{
		"lucro_liquido": ptr(4e9), "ebit": ptr(5e9),
	})

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "KEPL3"}, now)
	require.NoError(t, err)

	require.Contains(t, report.DetailMissingFor, domain.MetricID("ebit"))
	require.Contains(t, report.DetailMissingFor, domain.MetricID("lucro_liquido"))

	table := tableOf(t, report, domain.ClassStock)
	for _, c := range table.Columns {
		require.Nil(t, c.Cells["ebit"].Value, "%s ainda mostra EBIT", c.Ticker)
		require.Nil(t, c.Cells["lucro_liquido"].Value, "%s ainda mostra Lucro Líquido", c.Ticker)
	}
}

func TestCompare_DetailKeptWhenEveryTickerHasIt(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")
	for _, ticker := range []string{"WEGE3", "ROMI3", "KEPL3"} {
		seedDetail(t, db, ticker, map[domain.MetricID]*float64{
			"lucro_liquido": ptr(4e9), "ebit": ptr(5e9),
		})
	}

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "KEPL3"}, now)
	require.NoError(t, err)

	require.Empty(t, report.DetailMissingFor)
	require.NotNil(t, columnOf(t, tableOf(t, report, domain.ClassStock), "KEPL3").Cells["ebit"].Value)
}

func TestCompare_StructuralAbsenceDoesNotDropTheMetric(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3")
	seedBank(t, db, wege3Values())
	for _, ticker := range []string{"WEGE3", "ROMI3"} {
		seedDetail(t, db, ticker, map[domain.MetricID]*float64{
			"lucro_liquido": ptr(4e9), "ebit": ptr(5e9),
		})
	}
	seedDetail(t, db, "ITUB4", map[domain.MetricID]*float64{"lucro_liquido": ptr(40e9)})

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "ITUB4"}, now)
	require.NoError(t, err)

	require.NotContains(t, report.DetailMissingFor, domain.MetricID("ebit"))

	table := tableOf(t, report, domain.ClassStock)
	require.NotNil(t, columnOf(t, table, "WEGE3").Cells["ebit"].Value)
	require.Nil(t, columnOf(t, table, "ITUB4").Cells["ebit"].Value)
	require.NotEmpty(t, columnOf(t, table, "ITUB4").Cells["ebit"].NotApplicableReason)
}

func TestCompare_CarriesSelicBazinAndAlerts(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["dy"] = ptr(0.45)
	values["pl"] = ptr(2.5)
	values["cotacao"] = ptr(24.00)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	seedStocks(t, db, "ROMI3", "KEPL3")
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"BBAS3", "ROMI3", "KEPL3"}, now)
	require.NoError(t, err)

	require.NotNil(t, report.Header.SelicRate)
	require.InDelta(t, 0.14, *report.Header.SelicRate, 1e-9)

	bbas := columnOf(t, tableOf(t, report, domain.ClassStock), "BBAS3")
	require.NotNil(t, bbas.SelicDelta)
	require.InDelta(t, 0.31, *bbas.SelicDelta, 1e-9)

	require.NotNil(t, bbas.Bazin)
	require.Empty(t, bbas.Bazin.NotApplicableReason)
	require.InDelta(t, 20.0, bbas.Bazin.Ceiling, 1e-9)
	require.InDelta(t, 0.20, bbas.Bazin.PremiumDiscount, 1e-9)

	require.Len(t, bbas.Alerts, 5)
	require.Equal(t, evaluate.StatusFired, alertIn(t, bbas.Alerts, "ALERTA-01").Status)
}

func TestCompare_RunsWithoutTerminal(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"WEGE3", "ROMI3", "KEPL3"}, now)
	require.NoError(t, err)
	require.NotEmpty(t, report.Tables)
	require.NotEmpty(t, tableOf(t, report, domain.ClassStock).Columns)
}

func TestCompare_AllTickersInvalid(t *testing.T) {
	db := openTemp(t)

	report, err := app.Compare(t.Context(), db, loadCatalog(t),
		[]string{"AAAA3", "BBBB3", "CCCC3"}, now)
	require.NoError(t, err)
	require.Empty(t, report.Tables)
	require.Len(t, report.Invalid, 3)
}

func hasMetric(metrics []catalog.Metric, id domain.MetricID) bool {
	for _, m := range metrics {
		if m.ID == id {
			return true
		}
	}
	return false
}

func alertIn(t *testing.T, findings []evaluate.Finding, id string) evaluate.Finding {
	t.Helper()
	for _, f := range findings {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("alerta %s ausente", id)
	return evaluate.Finding{}
}
