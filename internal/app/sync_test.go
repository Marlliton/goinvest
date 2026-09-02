package app_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/collect"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/provider/fundamentus"
	"github.com/stretchr/testify/require"
)

const (
	stockFixture = "../provider/fundamentus/testdata/resultado_min.html"
	fiiFixture   = "../provider/fundamentus/testdata/fii_resultado_min.html"
)

var (
	stockTickers = []string{"WEGE3", "ITUB4", "CLAN3"}
	fiiTickers   = []string{"MXRF11", "ABCP11", "HGLG11"}
)

func newFundamentusStub(t *testing.T) *httptest.Server {
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
	mux.HandleFunc("/fii_resultado.php", serve(fiiFixture))

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestSyncReportsAssetCount(t *testing.T) {
	srv := newFundamentusStub(t)
	db := openTemp(t)

	client := fetch.NewClient(fetch.Config{RateEvery: time.Millisecond})
	p := fundamentus.NewProvider(client, srv.URL, time.Now)

	report, err := app.Sync(t.Context(), app.SyncConfig{
		Providers: map[domain.AssetClass]provider.UniverseProvider{
			domain.ClassStock: p,
			domain.ClassFII:   p,
		},
		DB: db,
	})
	require.NoError(t, err)

	require.Equal(t, collect.StatusOK, report.Stocks.Status)
	require.Equal(t, collect.StatusOK, report.FIIs.Status)
	require.Equal(t, len(stockTickers), report.Stocks.AssetCount)
	require.Equal(t, len(fiiTickers), report.FIIs.AssetCount)

	// Sem esta segunda contagem, o teste provaria só o parser: é ela que pega o
	// relatório afirmar um número que o banco não tem.
	var distinct int
	require.NoError(t, db.QueryRow(`SELECT COUNT(DISTINCT ticker) FROM asset`).Scan(&distinct))
	require.Equal(t, len(stockTickers)+len(fiiTickers), distinct)
}
