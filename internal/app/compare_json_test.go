package app_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func decodeJSON(t *testing.T, r app.CompareReport) map[string]any {
	t.Helper()
	raw, err := app.RenderCompareJSON(r)
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}

func firstColumn(t *testing.T, doc map[string]any) map[string]any {
	t.Helper()
	tables, ok := doc["tables"].([]any)
	require.True(t, ok, "tables ausente")
	require.NotEmpty(t, tables)
	columns, ok := tables[0].(map[string]any)["columns"].([]any)
	require.True(t, ok, "columns ausente")
	require.NotEmpty(t, columns)
	return columns[0].(map[string]any)
}

func metricDoc(t *testing.T, col map[string]any, id string) map[string]any {
	t.Helper()
	metrics, ok := col["metrics"].(map[string]any)
	require.True(t, ok, "metrics ausente")
	m, ok := metrics[id].(map[string]any)
	require.True(t, ok, "métrica %s ausente do contrato", id)
	return m
}

func TestRenderCompareJSON_SchemaVersion(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	doc := decodeJSON(t, compareOf(t, db, "WEGE3", "ROMI3", "KEPL3"))
	require.Equal(t, float64(1), doc["schema_version"])
}

func TestRenderCompareJSON_NumbersAreNumbers(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	col := firstColumn(t, decodeJSON(t, compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")))
	value := metricDoc(t, col, "pl")["value"]

	require.IsType(t, float64(0), value, "número JSON, nunca string formatada em pt-BR")
	require.InDelta(t, 30.0, value, 1e-9)

	equity := metricDoc(t, col, "patrim_liq")["value"]
	require.IsType(t, float64(0), equity)
	require.InDelta(t, 15e9, equity, 1, "sem abreviação de escala no contrato de máquina")
}

func TestRenderCompareJSON_FourAbsenceStates(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["cresc_rec_5a"] = nil
	values["ev_ebitda"] = nil
	seedStocks(t, db, "ROMI3", "KEPL3")
	seed(t, db, "ITUB4", domain.ClassStock, values)
	setIdentity(t, db, "ITUB4", "Financeiro", "Bancos", "Bancos")
	seedDetail(t, db, "ITUB4", map[domain.MetricID]*float64{"lucro_liquido": ptr(40e9)})

	col := firstColumn(t, decodeJSON(t, compareOf(t, db, "ITUB4", "ROMI3", "KEPL3")))

	require.Equal(t, "presente", metricDoc(t, col, "pl")["status"])
	require.Equal(t, "nao_informado", metricDoc(t, col, "cresc_rec_5a")["status"])
	require.Equal(t, "nao_avaliado", metricDoc(t, col, "ebit")["status"])

	notApplicable := metricDoc(t, col, "ev_ebitda")
	require.Equal(t, "nao_aplicavel", notApplicable["status"])
	require.NotEmpty(t, notApplicable["not_applicable_reason"])
}

func TestRenderCompareJSON_CarriesProvenance(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	col := firstColumn(t, decodeJSON(t, compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")))
	m := metricDoc(t, col, "pl")

	require.Equal(t, "fundamentus:resultado", m["source"])
	fetched, ok := m["fetched_at"].(string)
	require.True(t, ok, "fetched_at ausente")
	parsed, err := time.Parse(time.RFC3339, fetched)
	require.NoError(t, err, "data em RFC 3339")
	require.Equal(t, collectedAt.UTC(), parsed.UTC())
}

func TestRenderCompareJSON_AllFiveAlerts(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	col := firstColumn(t, decodeJSON(t, compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")))
	alerts, ok := col["alerts"].([]any)
	require.True(t, ok, "alerts ausente")
	require.Len(t, alerts, 5, "o não avaliado também é contrato")

	for _, a := range alerts {
		require.NotEmpty(t, a.(map[string]any)["status"])
	}
}

func TestRenderCompareJSON_BazinFullYears(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["cotacao"] = ptr(24.00)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	seedStocks(t, db, "ROMI3", "KEPL3")
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	col := firstColumn(t, decodeJSON(t, compareOf(t, db, "BBAS3", "ROMI3", "KEPL3")))
	bz, ok := col["bazin"].(map[string]any)
	require.True(t, ok, "bazin ausente")

	require.InDelta(t, 20.0, bz["ceiling"], 1e-9)
	require.Equal(t, float64(5), bz["years_used"])

	years, ok := bz["years"].([]any)
	require.True(t, ok, "a lista anual não é resumida no contrato")
	require.Len(t, years, 5)
	require.Equal(t, float64(2025), years[0].(map[string]any)["year"])
	require.InDelta(t, 1.20, years[0].(map[string]any)["total"], 1e-9)
}

func TestRenderCompareJSON_InvalidAndDetailMissing(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	doc := decodeJSON(t, compareOf(t, db, "WEGE3", "ROMI3", "KEPL3", "PETR4"))

	invalid, ok := doc["invalid"].([]any)
	require.True(t, ok)
	require.Len(t, invalid, 1)
	require.Equal(t, "PETR4", invalid[0].(map[string]any)["ticker"])
	require.NotEmpty(t, invalid[0].(map[string]any)["reason"])

	missing, ok := doc["detail_missing_for"].([]any)
	require.True(t, ok)
	require.Contains(t, missing, "ebit")
}

func TestRenderCompareJSON_EmptyReportIsValidJSON(t *testing.T) {
	var doc map[string]any
	raw, err := app.RenderCompareJSON(app.CompareReport{})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &doc))
	require.Equal(t, float64(1), doc["schema_version"])
}

// O contrato não pode nascer dependente da ordem de iteração de um mapa Go.
func TestRenderCompareJSON_IsDeterministic(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")
	report := compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")

	first, err := app.RenderCompareJSON(report)
	require.NoError(t, err)
	for range 5 {
		again, err := app.RenderCompareJSON(report)
		require.NoError(t, err)
		require.Equal(t, string(first), string(again))
	}
}
