package collect_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

var seededAt = time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

// A fonte de ações responde 200 com a fixture; a de FIIs responde 503.
func newSource(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/resultado.php", func(w http.ResponseWriter, _ *http.Request) {
		body, err := os.ReadFile(filepath.FromSlash(stockFixture))
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

	stocks, err := db.LatestMetrics(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.NotEmpty(t, stocks)

	fiis, err := db.LatestMetrics(t.Context(), "MXRF11")
	require.NoError(t, err)
	require.Contains(t, fiis, domain.MetricID("dy"))
	require.NotNil(t, fiis["dy"].Value)
	require.InDelta(t, 0.1234, *fiis["dy"].Value, 1e-9)
}
