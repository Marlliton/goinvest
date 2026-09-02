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

// As fixtures são lidas do testdata do provider em vez de copiadas: duas cópias
// divergem no dia em que a fonte mudar de layout.
const (
	stockFixture = "../provider/fundamentus/testdata/resultado_min.html"
	fiiFixture   = "../provider/fundamentus/testdata/fii_resultado_min.html"
)

// Os tickers de cada fixture, nomeados para que a contagem esperada possa ser
// conferida contra o arquivo em vez de contra um número solto.
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

	// A contagem relatada é uma afirmação sobre o banco, não sobre o parser: se
	// as duas divergirem, o relatório do sync estaria mentindo para o usuário.
	var distinct int
	require.NoError(t, db.QueryRow(`SELECT COUNT(DISTINCT ticker) FROM asset`).Scan(&distinct))
	require.Equal(t, len(stockTickers)+len(fiiTickers), distinct)
}
