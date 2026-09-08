package collect_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/collect"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/provider/fundamentus"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

// A fixture é a mesma que o provider usa, lida de lá em vez de copiada: duas
// cópias divergem no dia em que a fonte mudar de layout.
const stockFixture = "../provider/fundamentus/testdata/resultado_min.html"

// Fixture sintética: cotação, liquidez zerada e liquidez abaixo do piso, os três
// casos da régua.
const liquidityFixture = "../provider/fundamentus/testdata/resultado_liquidity.html"

var seededAt = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

// A fonte de ações responde 200 com a fixture; a de FIIs responde 503.
func newSource(t *testing.T) *httptest.Server {
	t.Helper()
	return newSourceFrom(t, stockFixture)
}

func newSourceFrom(t *testing.T, fixture string) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/resultado.php", func(w http.ResponseWriter, _ *http.Request) {
		body, err := os.ReadFile(filepath.FromSlash(fixture))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/fii_resultado.php", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func openDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// seedPreviousFIIRun grava uma coleta de FIIs bem-sucedida anterior, para que o
// teste possa provar que a falha de hoje não apagou o que já havia.
func seedPreviousFIIRun(t *testing.T, db *store.DB) {
	t.Helper()

	ctx := t.Context()
	require.NoError(t, db.UpsertAsset(ctx, "MXRF11", domain.ClassFII, "MAXI RENDA", seededAt))

	runID, err := db.StartRun(ctx, "fundamentus:fii_resultado")
	require.NoError(t, err)

	dy := 0.1234
	require.NoError(t, db.InsertObservations(ctx, runID, []domain.Observation{{
		Ticker:     "MXRF11",
		Metric:     "dy",
		PeriodKind: "spot",
		PeriodEnd:  seededAt,
		Value:      &dy,
		Unit:       domain.UnitPercent,
		Source:     "fundamentus:fii_resultado",
		FetchedAt:  seededAt,
	}}))
	require.NoError(t, db.FinishRun(ctx, runID, collect.StatusOK, 1, ""))
}

func latestRunStatus(t *testing.T, db *store.DB, source string) string {
	t.Helper()

	var status string
	require.NoError(t, db.QueryRow(
		`SELECT status FROM collection_run WHERE source = ? ORDER BY id DESC LIMIT 1`,
		source).Scan(&status))
	return status
}

func latestMetrics(t *testing.T, db *store.DB, ticker string) domain.MetricSet {
	t.Helper()
	a, found, err := db.GetAsset(t.Context(), ticker)
	require.NoError(t, err)
	require.True(t, found)
	set, err := db.LatestMetrics(t.Context(), a.AssetID, a.Ticker)
	require.NoError(t, err)
	return set
}

func TestSyncRegistersFractionalAliasForStocksOnly(t *testing.T) {
	srv := newSource(t)
	db := openDB(t)
	seedPreviousFIIRun(t, db)

	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	p := fundamentus.NewProvider(client, srv.URL, time.Now)

	_, err := collect.Sync(t.Context(), collect.Config{
		Providers: map[domain.AssetClass]provider.UniverseProvider{
			domain.ClassStock: p,
			domain.ClassFII:   p,
		},
		DB: db,
	})
	require.NoError(t, err)

	a, found, err := db.GetAsset(t.Context(), "WEGE3F")
	require.NoError(t, err)
	require.True(t, found, "ação ganha alias fracionário no ingest")
	require.Equal(t, "WEGE3", a.Ticker, "o alias resolve para o ativo canônico")

	canonical, _, err := db.GetAsset(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.Equal(t, canonical.AssetID, a.AssetID)

	_, found, err = db.GetAsset(t.Context(), "MXRF11F")
	require.NoError(t, err)
	require.False(t, found, "FII não tem mercado fracionário")
}

func syncStocks(t *testing.T, db *store.DB, srv *httptest.Server, now func() time.Time) {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	p := fundamentus.NewProvider(client, srv.URL, time.Now)

	_, err := collect.Sync(t.Context(), collect.Config{
		Providers: map[domain.AssetClass]provider.UniverseProvider{domain.ClassStock: p},
		DB:        db,
		Now:       now,
	})
	require.NoError(t, err)
}

func assetOf(t *testing.T, db *store.DB, ticker string) domain.Asset {
	t.Helper()
	a, found, err := db.GetAsset(t.Context(), ticker)
	require.NoError(t, err)
	require.True(t, found, ticker)
	return a
}

func TestSyncMarksInactiveByLiquidityRule(t *testing.T) {
	db := openDB(t)
	syncStocks(t, db, newSourceFrom(t, liquidityFixture), nil)

	require.False(t, assetOf(t, db, "DEAD3").IsActive, "liquidez zerada é morto, mesmo com cotação")
	require.False(t, assetOf(t, db, "ILIQ3").IsActive, "abaixo do piso é ilíquido")
	require.True(t, assetOf(t, db, "LIQ3").IsActive)

	require.NotNil(t, assetOf(t, db, "LIQ3").LastLiquidAt)
	require.Nil(t, assetOf(t, db, "DEAD3").LastLiquidAt, "nunca esteve líquido")
}

func TestSyncMarksInactiveByZeroQuote(t *testing.T) {
	db := openDB(t)
	syncStocks(t, db, newSource(t), nil)

	require.False(t, assetOf(t, db, "CLAN3").IsActive)
	require.True(t, assetOf(t, db, "WEGE3").IsActive)
}

func TestSyncPreservesLastLiquidAtWhenTickerGoesInactive(t *testing.T) {
	db := openDB(t)

	liquidAt := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	syncStocks(t, db, newSourceFrom(t, stockFixture), func() time.Time { return liquidAt })

	before := assetOf(t, db, "WEGE3")
	require.True(t, before.IsActive)
	require.NotNil(t, before.LastLiquidAt)

	dead := newSourceFrom(t, wege3DeadFixture(t))
	syncStocks(t, db, dead, func() time.Time { return liquidAt.AddDate(0, 1, 0) })

	after := assetOf(t, db, "WEGE3")
	require.False(t, after.IsActive)
	require.NotNil(t, after.LastLiquidAt, "a data do último dia líquido é preservada")
	require.Equal(t, before.LastLiquidAt.UTC(), after.LastLiquidAt.UTC())
}

func wege3DeadFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash(stockFixture))
	require.NoError(t, err)

	body := strings.Replace(string(raw), "354.626.000,00", "0,00", 1)
	require.NotEqual(t, string(raw), body, "a liquidez de WEGE3 mudou de formato na fixture")

	path := filepath.Join(t.TempDir(), "resultado_dead.html")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))
	return path
}

func TestSyncIsolatesPartialFailure(t *testing.T) {
	srv := newSource(t)
	db := openDB(t)
	seedPreviousFIIRun(t, db)

	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	p := fundamentus.NewProvider(client, srv.URL, time.Now)

	report, err := collect.Sync(context.Background(), collect.Config{
		Providers: map[domain.AssetClass]provider.UniverseProvider{
			domain.ClassStock: p,
			domain.ClassFII:   p,
		},
		DB: db,
	})

	// A falha de uma fonte é resultado, não erro: é o que mantém o exit 0 do
	// sync num cron.
	require.NoError(t, err)

	require.Equal(t, collect.StatusOK, report.Stocks.Status)
	require.Equal(t, "fundamentus:resultado", report.Stocks.Source)
	require.Equal(t, 3, report.Stocks.AssetCount)

	require.Equal(t, collect.StatusPartial, report.FIIs.Status)
	require.Equal(t, "fundamentus:fii_resultado", report.FIIs.Source)
	require.Contains(t, report.FIIs.Reason, "503")
	require.Zero(t, report.FIIs.AssetCount)

	require.Equal(t, collect.StatusOK, latestRunStatus(t, db, "fundamentus:resultado"))
	require.Equal(t, collect.StatusPartial, latestRunStatus(t, db, "fundamentus:fii_resultado"))

	stocks := latestMetrics(t, db, "WEGE3")
	require.NotEmpty(t, stocks)

	fiis := latestMetrics(t, db, "MXRF11")
	require.Contains(t, fiis, domain.MetricID("dy"))
	require.NotNil(t, fiis["dy"].Value)
	require.InDelta(t, 0.1234, *fiis["dy"].Value, 1e-9)
}

type fakeSelic struct {
	rate        float64
	referenceAt time.Time
	err         error
}

func (f fakeSelic) Name() string { return "bcb" }

func (f fakeSelic) Selic(context.Context, bool) (float64, time.Time, error) {
	return f.rate, f.referenceAt, f.err
}

func syncWithSelic(t *testing.T, db *store.DB, selic provider.SelicProvider) collect.Report {
	t.Helper()

	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	p := fundamentus.NewProvider(client, newHealthySource(t).URL, time.Now)

	report, err := collect.Sync(t.Context(), collect.Config{
		Providers: map[domain.AssetClass]provider.UniverseProvider{
			domain.ClassStock: p,
			domain.ClassFII:   p,
		},
		DB:    db,
		Selic: selic,
	})
	require.NoError(t, err)
	return report
}

// As duas classes respondem 200: aqui o único estágio que pode falhar é a Selic.
func newHealthySource(t *testing.T) *httptest.Server {
	t.Helper()

	serve := func(fixture string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			body, err := os.ReadFile(filepath.FromSlash(fixture))
			require.NoError(t, err)
			w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
			_, _ = w.Write(body)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/resultado.php", serve(stockFixture))
	mux.HandleFunc("/fii_resultado.php", serve("../provider/fundamentus/testdata/fii_resultado_min.html"))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSyncWritesSelicStage(t *testing.T) {
	db := openDB(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)

	report := syncWithSelic(t, db, fakeSelic{rate: 0.14, referenceAt: ref})

	require.Equal(t, collect.StatusOK, report.Selic.Status)
	require.Equal(t, "bcb", report.Selic.Source)

	value, referenceAt, _, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.True(t, found)
	require.InDelta(t, 0.14, *value, 1e-9)
	require.Equal(t, ref, referenceAt.UTC())
}

func TestSyncIsolatesSelicFailure(t *testing.T) {
	db := openDB(t)
	ref := time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC)

	require.NoError(t, db.PutSelic(t.Context(), 0.14, ref, ref))

	report := syncWithSelic(t, db, fakeSelic{err: errors.New("bcb fora do ar")})

	require.Equal(t, collect.StatusOK, report.Stocks.Status)
	require.Equal(t, collect.StatusOK, report.FIIs.Status)
	require.Equal(t, collect.StatusPartial, report.Selic.Status)
	require.Contains(t, report.Selic.Reason, "bcb fora do ar")

	value, _, _, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.True(t, found, "estágio que falha preserva a Selic anterior")
	require.InDelta(t, 0.14, *value, 1e-9)
}

// Sem provider registrado o estágio não existe: o campo fica zerado, e o sync
// não inventa uma falha.
func TestSyncWithoutSelicProvider(t *testing.T) {
	db := openDB(t)

	report := syncWithSelic(t, db, nil)

	require.Empty(t, report.Selic.Status)
	_, _, _, found, err := db.GetSelic(t.Context())
	require.NoError(t, err)
	require.False(t, found)
}
