package app_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/config"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

var update = flag.Bool("update", false, "regrava os arquivos golden")

var (
	collectedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	fixedNow    = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	defaultTax  = config.Result{Tributacao: config.Default(), Path: "config.toml", FromFile: false}
)

func now() time.Time { return fixedNow }

func ptr(v float64) *float64 { return &v }

func openTemp(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func seed(t *testing.T, db *store.DB, ticker string, class domain.AssetClass, values map[domain.MetricID]*float64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.UpsertAsset(ctx, ticker, class, ticker+" S.A.", collectedAt))

	runID, err := db.StartRun(ctx, "fundamentus:resultado")
	require.NoError(t, err)

	obs := make([]domain.Observation, 0, len(values))
	for id, v := range values {
		obs = append(obs, domain.Observation{
			Ticker:     ticker,
			Metric:     id,
			PeriodKind: "spot",
			PeriodEnd:  collectedAt.Truncate(24 * time.Hour),
			Value:      v,
			Unit:       domain.UnitRatio,
			Source:     "fundamentus:resultado",
			FetchedAt:  collectedAt,
		})
	}
	require.NoError(t, db.InsertObservations(ctx, runID, obs))
	require.NoError(t, db.FinishRun(ctx, runID, "ok", len(obs), ""))

	if class == domain.ClassStock {
		a, found, err := db.GetAsset(ctx, ticker)
		require.NoError(t, err)
		require.True(t, found)
		require.NoError(t, db.UpsertAssetAlias(ctx, identity.FractionalAlias(ticker), a.AssetID))
	}
}

func loadCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Load()
	require.NoError(t, err)
	return cat
}

func wege3Values() map[domain.MetricID]*float64 {
	return map[domain.MetricID]*float64{
		"cotacao":         ptr(52.30),
		"liq_2meses":      ptr(180_000_000),
		"pl":              ptr(30.0),
		"pvp":             ptr(9.5),
		"psr":             ptr(6.0),
		"p_ativo":         ptr(4.0),
		"p_cap_giro":      ptr(8.0),
		"p_ebit":          ptr(25.0),
		"p_ativ_circ_liq": ptr(-3.0),
		"ev_ebit":         ptr(24.0),
		"ev_ebitda":       ptr(20.0),
		"mrg_bruta":       ptr(0.32),
		"mrg_ebit":        ptr(0.20),
		"mrg_liq":         ptr(0.17),
		"roic":            ptr(0.28),
		"roe":             ptr(0.25),
		// Métrica sem sentinela: este zero é legítimo e precisa sair como número.
		"cresc_rec_5a": ptr(0.0),
		"liq_corr":     ptr(2.1),
		"patrim_liq":   ptr(15e9),
		"dl_patrim":    ptr(0.10),
		"dy":           ptr(0.012),
	}
}

func TestShowNeverReachesTheNetwork(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.NotZero(t, report.Header.FetchedAt)
	require.Equal(t, collectedAt, report.Header.FetchedAt)
	require.Nil(t, report.Header.ReferenceAt)
	require.NotContains(t, app.RenderText(report), collectedAt.Format("02/01/2006"),
		"a data de coleta não pode aparecer como data de balanço")
}

func TestShowResolvesFractionalTicker(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "PETR4", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "PETR4F", defaultTax, now)
	require.NoError(t, err)
	require.Equal(t, "PETR4", report.Ticker, "o fracionário devolve a análise do canônico")

	_, err = app.Show(t.Context(), db, loadCatalog(t), "MXRF11F", defaultTax, now)
	require.ErrorIs(t, err, app.ErrNoData, "FII não tem alias fracionário")
}

func TestShowInactiveAsset(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "DEAD3", domain.ClassStock, wege3Values())

	a, _, err := db.GetAsset(t.Context(), "DEAD3")
	require.NoError(t, err)
	lastLiquid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, true, lastLiquid))
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, false, fixedNow))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "DEAD3", defaultTax, now)
	require.NoError(t, err)
	require.True(t, report.Header.Inactive)
	require.NotNil(t, report.Header.LastLiquidAt)

	text := app.RenderText(report)
	require.Contains(t, text, "15/07/2026")
	require.Contains(t, text, "fora de rankings")
	require.Contains(t, text, "R$ 52,30", "o último retrato continua visível")
}

func TestShowInactiveAssetNeverSeenLiquid(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "DEAD3", domain.ClassStock, wege3Values())

	a, _, err := db.GetAsset(t.Context(), "DEAD3")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, false, fixedNow))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "DEAD3", defaultTax, now)
	require.NoError(t, err)
	require.Nil(t, report.Header.LastLiquidAt)
	require.Contains(t, app.RenderText(report), "sem liquidez registrada")
}

func setIdentity(t *testing.T, db *store.DB, ticker, sector, subsector, segment string) {
	t.Helper()
	a, found, err := db.GetAsset(t.Context(), ticker)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, db.UpdateAssetIdentities(t.Context(), []store.AssetIdentityUpdate{{
		AssetID: a.AssetID, Sector: sector, Subsector: subsector, Segment: segment,
		SectorSrc: "b3", UpdatedAt: collectedAt,
	}}))
}

func TestShowSectorFromRegistry(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores. Compressores e Outros")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.Equal(t, "Bens Industriais", report.Header.Sector)
	require.Contains(t, app.RenderText(report),
		"Setor: Bens Industriais / Máquinas e Equipamentos / Motores. Compressores e Outros")
}

func TestShowSectorSingleLevelTaxonomy(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "MXRF11", domain.ClassFII, wege3Values())
	setIdentity(t, db, "MXRF11", "Shoppings", "", "")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "MXRF11", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "Setor: Shoppings\n")
	require.NotContains(t, text, " / ")
}

func TestShowSectorUnknownWithoutRegistry(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.Contains(t, app.RenderText(report), "Setor: desconhecido")
}

func TestShowWarnsWhenClassRegistryIncomplete(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	seed(t, db, "ITUB4", domain.ClassStock, wege3Values())
	seed(t, db, "MXRF11", domain.ClassFII, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.Equal(t, 1, report.Header.IncompleteRegistry)
	require.Equal(t, 2, report.Header.TotalInClass, "a contagem é da classe do ativo, não do mercado")
	require.Contains(t, app.RenderText(report), "cadastro incompleto: 1 de 2")
}

func TestShowOmitsWarningWhenRegistryComplete(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.NotContains(t, app.RenderText(report), "cadastro incompleto")
}

func TestShowOmitsWarningWhenOnlyIlliquidAssetsLackSector(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores")
	seed(t, db, "DEAD3", domain.ClassStock, wege3Values())
	dead3, _, err := db.GetAsset(t.Context(), "DEAD3")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), dead3.AssetID, false, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)
	require.NotContains(t, app.RenderText(report), "cadastro incompleto")
}

func seedPeerGroup(t *testing.T, db *store.DB, tickers []string, sector, subsector, segment string, pl []float64) {
	t.Helper()
	for i, ticker := range tickers {
		seed(t, db, ticker, domain.ClassStock, map[domain.MetricID]*float64{
			"cotacao": ptr(10), "pl": ptr(pl[i]),
		})
		setIdentity(t, db, ticker, sector, subsector, segment)
		a, _, err := db.GetAsset(t.Context(), ticker)
		require.NoError(t, err)
		require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, true, collectedAt))
	}
	require.NoError(t, db.RecomputeSectorStats(t.Context(),
		[]store.MetricRule{{MetricID: "pl"}}, collectedAt))
}

func TestShowRendersPercentileAndPeerGroup(t *testing.T) {
	db := openTemp(t)
	seedPeerGroup(t, db,
		[]string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"},
		"Bens Industriais", "Máquinas", "Motores",
		[]float64{10, 12, 14, 16, 18})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "CCCC3", defaultTax, now)
	require.NoError(t, err)
	require.Equal(t, "Motores", report.Header.PeerGroupLabel)
	require.Equal(t, 5, report.Header.PeerGroupN)

	text := app.RenderText(report)
	require.Contains(t, text, "Comparado com Motores (5 papéis líquidos)")
	require.Contains(t, text, " · p60 · n=5", "3 de 5 valores são <= 14")
	require.NotContains(t, text, "Cotação: R$ 10,00 · p", "cotação não tem percentil declarado")
}

func TestShowInactiveAssetHasNoPercentile(t *testing.T) {
	db := openTemp(t)
	seedPeerGroup(t, db,
		[]string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"},
		"Bens Industriais", "Máquinas", "Motores",
		[]float64{10, 12, 14, 16, 18})

	a, _, err := db.GetAsset(t.Context(), "CCCC3")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, false, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "CCCC3", defaultTax, now)
	require.NoError(t, err)
	require.Empty(t, report.Header.PeerGroupLabel)

	text := app.RenderText(report)
	require.NotContains(t, text, " · p")
	require.NotContains(t, text, "Comparado com")
}

func TestShowReturnsErrNoDataForUnknownTicker(t *testing.T) {
	db := openTemp(t)

	_, err := app.Show(t.Context(), db, loadCatalog(t), "NADA4", defaultTax, now)
	require.ErrorIs(t, err, app.ErrNoData)
}

func TestShowReturnsErrNoDataForAssetNeverCollected(t *testing.T) {
	db := openTemp(t)
	require.NoError(t, db.UpsertAsset(t.Context(), "VALE3", domain.ClassStock, "VALE S.A.", collectedAt))

	_, err := app.Show(t.Context(), db, loadCatalog(t), "VALE3", defaultTax, now)
	require.ErrorIs(t, err, app.ErrNoData)
}

func TestShowGoldenOutput(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_wege3.txt", text)

	require.Contains(t, text, "0,00%", "zero legítimo aparece como número")
	catalogSection := text[strings.Index(text, "\nCotação\n"):]
	require.NotContains(t, catalogSection, "—", "nenhum insumo de WEGE3 está ausente")
	require.Contains(t, text, "ƒ", "os derivados saudáveis aparecem marcados")
	require.Contains(t, text, "(DY × P/L)", "a fórmula do derivado fica visível")
}

func TestShowGoldenOutputSuspectInput(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["ev_ebitda"] = nil
	seed(t, db, "ITUB4", domain.ClassStock, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_itub4.txt", text)

	require.Contains(t, text, "—", "a fonte informou ausência de EV/EBITDA")
	require.NotContains(t, text, "Dív.Líq./EBITDA", "derivado sobre insumo ausente não vira linha")
}

func TestShowGoldenOutputFII(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao":        ptr(9.87),
		"liquidez_fii":   ptr(4_500_000),
		"pvp":            ptr(1.02),
		"valor_mercado":  ptr(3_200_000_000),
		"qtd_imoveis":    ptr(0),
		"preco_m2":       ptr(0),
		"aluguel_m2":     ptr(0),
		"cap_rate":       ptr(0),
		"vacancia_media": ptr(0),
		"dy":             ptr(0.132),
		"ffo_yield":      ptr(0.128),
	})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "MXRF11", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_mxrf11.txt", text)

	require.NotContains(t, text, "P/L", "métrica que não se aplica à classe some da tela")
}

func TestRenderWarnsWhenDataIsStale(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	longAfterCollection := func() time.Time { return collectedAt.Add(30 * 24 * time.Hour) }

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, longAfterCollection)
	require.NoError(t, err)
	require.True(t, report.Header.Stale)
	require.Contains(t, app.RenderText(report), "⚠ dado de 01/09 · rode 'goinvest sync'")
}

func requireGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "golden", name)
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
		return
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err, "golden ausente: rode `go test ./internal/app/... -update`")
	require.Equal(t, string(want), got)
}

func lineOf(t *testing.T, r app.Report, id domain.MetricID) app.LineView {
	t.Helper()
	for _, b := range r.Blocks {
		for _, l := range b.Lines {
			if l.MetricID == id {
				return l
			}
		}
	}
	t.Fatalf("métrica %s não está no relatório", id)
	return app.LineView{}
}

func TestShowAnchorsDividendYieldOnSelic(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["dy"] = ptr(0.06)
	seed(t, db, "WEGE3", domain.ClassStock, values)

	referenceAt := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.PutSelic(t.Context(), 0.14, referenceAt, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.NotNil(t, report.Header.SelicRate)
	require.InDelta(t, 0.14, *report.Header.SelicRate, 1e-9)
	require.NotNil(t, report.Header.SelicAt)
	require.Equal(t, referenceAt, report.Header.SelicAt.UTC())

	dy := lineOf(t, report, "dy")
	require.NotNil(t, dy.SelicDelta)
	require.InDelta(t, -0.08, *dy.SelicDelta, 1e-9)

	require.Nil(t, lineOf(t, report, "roe").SelicDelta, "só o DY é ancorado")

	text := app.RenderText(report)
	require.Contains(t, text, "Selic: 14,00% bruto (16/09/2026)")
	require.Contains(t, text, "DY−Selic: -8,00pp")
}

func TestShowSelicUnknownIsNotEvaluated(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["dy"] = ptr(0.06)
	seed(t, db, "WEGE3", domain.ClassStock, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.Nil(t, report.Header.SelicRate)
	require.Nil(t, report.Header.SelicAt)
	require.Nil(t, lineOf(t, report, "dy").SelicDelta)

	text := app.RenderText(report)
	require.Contains(t, text, "Selic: — (Selic desconhecida; rode 'goinvest sync')")
	require.NotContains(t, text, "DY−Selic")
}

func TestShowAnchorIsAbsentWhenDividendYieldIsAbsent(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["dy"] = nil
	seed(t, db, "WEGE3", domain.ClassStock, values)
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.Nil(t, lineOf(t, report, "dy").SelicDelta)
	require.NotContains(t, app.RenderText(report), "DY−Selic")
}

func hasLine(r app.Report, id domain.MetricID) bool {
	for _, b := range r.Blocks {
		for _, l := range b.Lines {
			if l.MetricID == id {
				return true
			}
		}
	}
	return false
}

func seedDetail(t *testing.T, db *store.DB, ticker string, values map[domain.MetricID]*float64) {
	t.Helper()
	ctx := t.Context()

	runID, err := db.StartRun(ctx, "fundamentus:detalhes")
	require.NoError(t, err)

	obs := make([]domain.Observation, 0, len(values))
	for id, v := range values {
		obs = append(obs, domain.Observation{
			Ticker:     ticker,
			Metric:     id,
			PeriodKind: "ttm",
			PeriodEnd:  collectedAt.Truncate(24 * time.Hour),
			Value:      v,
			Unit:       domain.UnitBRL,
			Source:     "fundamentus:detalhes",
			FetchedAt:  collectedAt,
		})
	}
	require.NoError(t, db.InsertObservations(ctx, runID, obs))
	require.NoError(t, db.FinishRun(ctx, runID, "ok", len(obs), ""))
}

func seedBank(t *testing.T, db *store.DB, values map[domain.MetricID]*float64) {
	t.Helper()
	seed(t, db, "ITUB4", domain.ClassStock, values)
	setIdentity(t, db, "ITUB4", "Financeiro", "Intermediários Financeiros", "Bancos")
}

func TestShow_BankSentinel(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["ev_ebitda"] = nil
	seedBank(t, db, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", defaultTax, now)
	require.NoError(t, err)

	line := lineOf(t, report, "ev_ebitda")
	require.Nil(t, line.Value)
	require.NotEmpty(t, line.NotApplicableReason)
	require.Contains(t, line.NotApplicableReason, "Banco")
}

func TestShow_BankSentinelDoesNotTouchOtherSegments(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["ev_ebitda"] = nil
	seed(t, db, "WEGE3", domain.ClassStock, values)
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores. Compressores e Outros")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	line := lineOf(t, report, "ev_ebitda")
	require.Nil(t, line.Value)
	require.Empty(t, line.NotApplicableReason, "ausência fora do segmento sentinela continua sendo '—'")
}

func TestShow_EBITStructurallyAbsent(t *testing.T) {
	db := openTemp(t)
	seedBank(t, db, wege3Values())
	seedDetail(t, db, "ITUB4", map[domain.MetricID]*float64{"lucro_liquido": ptr(30e9)})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", defaultTax, now)
	require.NoError(t, err)

	line := lineOf(t, report, "ebit")
	require.Nil(t, line.Value)
	require.NotEmpty(t, line.NotApplicableReason)
}

func TestShow_EBITNeverCollected(t *testing.T) {
	db := openTemp(t)
	seedBank(t, db, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", defaultTax, now)
	require.NoError(t, err)

	require.False(t, hasLine(report, "ebit"),
		"sem o detalhe coletado não há como afirmar que a fonte não publica")
}

func TestShow_NegativeEquity(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["patrim_liq"] = ptr(-1_000_000_000)
	seed(t, db, "WEGE3", domain.ClassStock, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	line := lineOf(t, report, "pvp")
	require.Nil(t, line.Value, "o múltiplo calculado não é apresentado como comparável")
	require.NotEmpty(t, line.NotApplicableReason)
	require.Contains(t, line.NotApplicableReason, "Patrimônio")
}

func TestShow_PositiveEquityKeepsThePriceToBookValue(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	line := lineOf(t, report, "pvp")
	require.NotNil(t, line.Value)
	require.Empty(t, line.NotApplicableReason)
}

func TestShow_LineViewCarriesReferenceAt(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	referenceAt := collectedAt.Add(-45 * 24 * time.Hour)
	runID, err := db.StartRun(t.Context(), "fundamentus:detalhes")
	require.NoError(t, err)
	require.NoError(t, db.InsertObservations(t.Context(), runID, []domain.Observation{{
		Ticker:      "WEGE3",
		Metric:      "ebit",
		PeriodKind:  "ttm",
		PeriodEnd:   collectedAt.Truncate(24 * time.Hour),
		Value:       ptr(1_000_000_000),
		Unit:        domain.UnitBRL,
		Source:      "fundamentus:detalhes",
		ReferenceAt: &referenceAt,
		FetchedAt:   collectedAt,
	}}))
	require.NoError(t, db.FinishRun(t.Context(), runID, "ok", 1, ""))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.Nil(t, lineOf(t, report, "pl").ReferenceAt)
	ebit := lineOf(t, report, "ebit")
	require.NotNil(t, ebit.ReferenceAt)
	require.Equal(t, referenceAt, ebit.ReferenceAt.UTC())
}

func TestShowExpectedReturnDecomposition(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.NotNil(t, report.ExpectedReturn)
	v := report.ExpectedReturn
	require.Empty(t, v.NotApplicableReason)
	require.InDelta(t, 1.0/30.0, v.EarningsYield, 1e-9)
	require.InDelta(t, 0.012, v.Distributed, 1e-9)
	require.InDelta(t, 1-0.012*30.0, v.Retained, 1e-9)
	require.InDelta(t, 0.25*(1-0.012*30.0), v.ImpliedGrowth, 1e-9)
	require.InDelta(t, 0.012+0.25*(1-0.012*30.0), v.TotalReturn, 1e-9)
	require.InDelta(t, 30.0, v.PaybackYears, 1e-9)

	require.Contains(t, app.RenderText(report), "Retorno esperado")
}

func TestShowExpectedReturnNotApplicableForFII(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87),
		"pvp":     ptr(1.02),
		"dy":      ptr(0.132),
	})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "MXRF11", defaultTax, now)
	require.NoError(t, err)

	require.NotNil(t, report.ExpectedReturn)
	require.NotEmpty(t, report.ExpectedReturn.NotApplicableReason)
	require.Zero(t, report.ExpectedReturn.TotalReturn)
}

func TestShowExpectedReturnNotApplicableWhenLossMaking(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["pl"] = ptr(-3.0)
	seed(t, db, "WEGE3", domain.ClassStock, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	require.NotNil(t, report.ExpectedReturn)
	require.Contains(t, report.ExpectedReturn.NotApplicableReason, "Prejuízo")
}

func TestShowGoldenOutputBank(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["ev_ebitda"] = nil
	seedBank(t, db, values)
	seedDetail(t, db, "ITUB4", map[domain.MetricID]*float64{"lucro_liquido": ptr(35e9)})

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_itub4_banco.txt", text)

	require.Contains(t, text, "não se aplica", "a legenda ganha o quarto estado")
	require.NotContains(t, text, "EV/EBITDA: —",
		"o que a fonte nunca publicaria não pode sair como ausência comum")
}

func TestShowGordonCeilingWithSensitivity(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, map[domain.MetricID]*float64{
		"cotacao":      ptr(24.00),
		"pl":           ptr(8.0),
		"dy":           ptr(0.06),
		"roe":          ptr(0.18),
		"cresc_rec_5a": ptr(0.05),
	})
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.NotNil(t, report.Bazin.Gordon)
	require.Empty(t, report.Bazin.Gordon.NotApplicableReason)

	payout := 0.06 * 8.0
	g := 0.18 * (1 - payout)
	required := 0.14 + 0.06
	dividendPerShare := 0.06 * 24.00
	wantCeiling := dividendPerShare / (required - g)
	require.InDelta(t, wantCeiling, report.Bazin.Gordon.Ceiling, 1e-9)
	require.GreaterOrEqual(t, len(report.Bazin.Gordon.Sensitivity), 2)

	text := app.RenderText(report)
	require.Contains(t, text, "Faixa:")
	require.Contains(t, text, "Gordon:")
}

func TestShowGordonNotApplicableWhenGrowthExceedsRequired(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, map[domain.MetricID]*float64{
		"cotacao": ptr(24.00),
		"pl":      ptr(10.0),
		"dy":      ptr(0.01),
		"roe":     ptr(0.30),
	})
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.NotNil(t, report.Bazin.Gordon)
	require.Contains(t, report.Bazin.Gordon.NotApplicableReason, "estoura")

	require.Contains(t, app.RenderText(report), "Gordon não aplicável")
}

func TestShowGordonNotApplicableForFII(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87),
		"pvp":     ptr(1.02),
		"dy":      ptr(0.132),
	})
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "MXRF11", 2021, 2025, 0.80)

	report := bazinReport(t, db, "MXRF11")
	require.Empty(t, report.Bazin.NotApplicableReason)
	require.NotZero(t, report.Bazin.Ceiling)
	require.NotNil(t, report.Bazin.Gordon)
	require.Contains(t, report.Bazin.Gordon.NotApplicableReason, "D-101")
}

func TestShowDYMedianAndAtypicalYear(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2021, 2024, 1.20)
	seedDividends(t, db, "BBAS3",
		event(exOn(2025, time.March, 10), domain.DividendCash, 1.10, 1),
		event(exOn(2025, time.September, 10), domain.DividendCash, 0.10, 1))

	report := bazinReport(t, db, "BBAS3")
	require.NotNil(t, report.Bazin.MedianDividendYield)

	text := app.RenderText(report)
	require.Contains(t, text, "ano 2025 concentrado")
	require.Contains(t, text, "DY 12m:")
	require.Contains(t, text, "DY mediano 5a:")
}

func TestShowOpportunityCostAllThreeAnchors(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	require.NoError(t, db.PutCDI(t.Context(), 0.13, collectedAt, collectedAt))
	require.NoError(t, db.PutTesouroIPCA10y(t.Context(), 0.07, collectedAt, collectedAt))
	require.NoError(t, db.PutFocusIPCA12m(t.Context(), 0.045, collectedAt, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "Custo de oportunidade")
	require.Contains(t, text, "Selic: 14,00% bruto")
	require.Contains(t, text, "CDI: 13,00% bruto")
	require.Contains(t, text, "Tesouro IPCA+ (~10 anos): 11,50% bruto")
	require.Contains(t, text, "IPCA+ 7,00% com inflação esperada de 4,50% = 11,50% nominal")
}

func TestShowOpportunityCostAnchorMissing(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	require.NoError(t, db.PutCDI(t.Context(), 0.13, collectedAt, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "Selic: 14,00% bruto")
	require.Contains(t, text, "CDI: 13,00% bruto")
	require.Contains(t, text, "Tesouro IPCA+ (~10 anos): — (Tesouro IPCA+ (~10 anos) desconhecida; rode 'goinvest sync')")
}

func TestShowOpportunityCostFocusMissing(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	require.NoError(t, db.PutTesouroIPCA10y(t.Context(), 0.07, collectedAt, collectedAt))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", defaultTax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "Tesouro IPCA+ (~10 anos): — (Tesouro IPCA+ desconhecido: falta a expectativa de inflação (Focus) para nominalizar)")
	require.NotContains(t, text, "7,00% bruto")
}

func TestShowTaxPremiseFromConfigFile(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	tax := config.Result{
		FromFile:   true,
		Path:       "/tmp/x/config.toml",
		Tributacao: config.Tributacao{AliquotaAtivo: 0.10, AliquotaRendaFixa: 0.15},
	}

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", tax, now)
	require.NoError(t, err)

	text := app.RenderText(report)
	require.Contains(t, text, "10,00%")
	require.Contains(t, text, "config em /tmp/x/config.toml")
}
