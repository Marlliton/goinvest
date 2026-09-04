package app_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

// Regravar sem ler o diff transforma o golden em carimbo do bug.
var update = flag.Bool("update", false, "regrava os arquivos golden")

var (
	collectedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	fixedNow    = time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
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

// Nenhum httptest.Server sobe aqui: se Show discasse rede, não haveria endpoint
// de pé, e o teste travaria em vez de passar em silêncio.
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

	// Reproduz o que o sync deixa gravado: ação ganha alias fracionário.
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

// Percentual é fração em todo o pipeline: 0.25 é 25%.
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

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
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

	report, err := app.Show(t.Context(), db, loadCatalog(t), "PETR4F", now)
	require.NoError(t, err)
	require.Equal(t, "PETR4", report.Ticker, "o fracionário devolve a análise do canônico")

	_, err = app.Show(t.Context(), db, loadCatalog(t), "MXRF11F", now)
	require.ErrorIs(t, err, app.ErrNoData, "FII não tem alias fracionário")
}

// Papel sem liquidez continua mostrando os números (é o último retrato), mas
// avisado, para não ser lido como comparável.
func TestShowInactiveAsset(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "DEAD3", domain.ClassStock, wege3Values())

	a, _, err := db.GetAsset(t.Context(), "DEAD3")
	require.NoError(t, err)
	lastLiquid := time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, true, lastLiquid))
	require.NoError(t, db.UpdateAssetLiquidity(t.Context(), a.AssetID, false, fixedNow))

	report, err := app.Show(t.Context(), db, loadCatalog(t), "DEAD3", now)
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

	report, err := app.Show(t.Context(), db, loadCatalog(t), "DEAD3", now)
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

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)
	require.Equal(t, "Bens Industriais", report.Header.Sector)
	require.Contains(t, app.RenderText(report),
		"Setor: Bens Industriais / Máquinas e Equipamentos / Motores. Compressores e Outros")
}

// Setor ausente é dito, não escondido: três barras soltas dariam a impressão
// de que a fonte respondeu vazio.
func TestShowSectorUnknownWithoutRegistry(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)
	require.Contains(t, app.RenderText(report), "Setor: desconhecido")
}

func TestShowWarnsWhenClassRegistryIncomplete(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	seed(t, db, "ITUB4", domain.ClassStock, wege3Values())
	seed(t, db, "MXRF11", domain.ClassFII, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)
	require.Equal(t, 1, report.Header.IncompleteRegistry)
	require.Equal(t, 2, report.Header.TotalInClass, "a contagem é da classe do ativo, não do mercado")
	require.Contains(t, app.RenderText(report), "cadastro incompleto: 1 de 2")
}

func TestShowOmitsWarningWhenRegistryComplete(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	setIdentity(t, db, "WEGE3", "Bens Industriais", "Máquinas e Equipamentos", "Motores")

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)
	require.NotContains(t, app.RenderText(report), "cadastro incompleto")
}

func TestShowReturnsErrNoDataForUnknownTicker(t *testing.T) {
	db := openTemp(t)

	_, err := app.Show(t.Context(), db, loadCatalog(t), "NADA4", now)
	require.ErrorIs(t, err, app.ErrNoData)
}

func TestShowReturnsErrNoDataForAssetNeverCollected(t *testing.T) {
	db := openTemp(t)
	require.NoError(t, db.UpsertAsset(t.Context(), "VALE3", domain.ClassStock, "VALE S.A.", collectedAt))

	_, err := app.Show(t.Context(), db, loadCatalog(t), "VALE3", now)
	require.ErrorIs(t, err, app.ErrNoData)
}

func TestShowGoldenOutput(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_wege3.txt", text)

	require.Contains(t, text, "0,00%", "zero legítimo aparece como número")
	require.NotContains(t, text, "—", "nenhum insumo de WEGE3 está ausente")
	require.Contains(t, text, "ƒ", "os derivados saudáveis aparecem marcados")
	require.Contains(t, text, "(DY × P/L)", "a fórmula do derivado fica visível")
}

func TestShowGoldenOutputSuspectInput(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	// Banco: a fonte não publica EV/EBITDA, e sem ele DL/EBITDA não sai.
	values["ev_ebitda"] = nil
	seed(t, db, "ITUB4", domain.ClassStock, values)

	report, err := app.Show(t.Context(), db, loadCatalog(t), "ITUB4", now)
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

	report, err := app.Show(t.Context(), db, loadCatalog(t), "MXRF11", now)
	require.NoError(t, err)

	text := app.RenderText(report)
	requireGolden(t, "show_mxrf11.txt", text)

	require.NotContains(t, text, "P/L", "métrica que não se aplica à classe some da tela")
}

func TestRenderWarnsWhenDataIsStale(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "WEGE3", domain.ClassStock, wege3Values())
	longAfterCollection := func() time.Time { return collectedAt.Add(30 * 24 * time.Hour) }

	report, err := app.Show(t.Context(), db, loadCatalog(t), "WEGE3", longAfterCollection)
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
