package fundamentus_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/fundamentus"
	"github.com/stretchr/testify/require"
)

// Ao contrário das tabelas bulk, o mesmo path serve conteúdo diferente por
// ativo, então o papel entra na chave.
var tickerFixtures = map[string]string{
	"/detalhes.php?WEGE3":       "detalhes_wege3.html",
	"/detalhes.php?ITUB4":       "detalhes_itub4.html",
	"/detalhes.php?MXRF11":      "detalhes_mxrf11.html",
	"/detalhes.php?QUEBRA3":     "resultado_min.html",
	"/proventos.php?BBAS3":      "proventos_bbas3.html",
	"/proventos.php?ITSA4":      "proventos_itsa4.html",
	"/proventos.php?PETR4":      "proventos_petr4.html",
	"/proventos.php?QUEBRA3":    "detalhes_wege3.html",
	"/proventos.php?ZEROF3":     "proventos_zerofactor.html",
	"/fii_proventos.php?MXRF11": "fii_proventos_mxrf11.html",
}

func newTickerProvider(t *testing.T) *fundamentus.Provider {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := tickerFixtures[r.URL.Path+"?"+r.URL.Query().Get("papel")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return fundamentus.NewProvider(client, srv.URL, time.Now)
}

func detail(t *testing.T, ticker string, class domain.AssetClass) domain.MetricSet {
	t.Helper()
	set, err := newTickerProvider(t).Detail(t.Context(), ticker, class, false)
	require.NoError(t, err)
	return set
}

func TestParseDetail_Industrial(t *testing.T) {
	wege3 := detail(t, "WEGE3", domain.ClassStock)

	want := map[domain.MetricID]float64{
		"ebit":          8_400_030_000,
		"lucro_liquido": 6_254_050_000,
	}
	for id, want := range want {
		obs, ok := wege3[id]
		require.True(t, ok, "métrica %q ausente", id)
		require.NotNil(t, obs.Value, "métrica %q veio nula", id)
		require.InDelta(t, want, *obs.Value, 1e-9, "métrica %q", id)
	}
}

// Ler a coluna de 3 meses passaria despercebido: o número existe e é plausível,
// só responde a outra pergunta.
func TestParseDetailReadsTwelveMonthsColumn(t *testing.T) {
	wege3 := detail(t, "WEGE3", domain.ClassStock)

	require.NotEqual(t, 2_107_770_000.0, *wege3["ebit"].Value)
	require.NotEqual(t, 1_558_630_000.0, *wege3["lucro_liquido"].Value)
}

// Banco não publica EBIT nesta fonte, e quem lê precisa distinguir isso de "a
// fonte publicou e o valor é nulo".
func TestParseDetail_Bank(t *testing.T) {
	itub4 := detail(t, "ITUB4", domain.ClassStock)

	require.Contains(t, itub4, domain.MetricID("lucro_liquido"))
	require.NotContains(t, itub4, domain.MetricID("ebit"))
}

func TestParseDetail_FII(t *testing.T) {
	mxrf11 := detail(t, "MXRF11", domain.ClassFII)

	want := map[domain.MetricID]float64{
		"receita":          526_490_000,
		"venda_ativos":     51_296_600,
		"ffo":              483_105_000,
		"rend_distribuido": 497_112_000,
	}
	for id, want := range want {
		obs, ok := mxrf11[id]
		require.True(t, ok, "métrica %q ausente", id)
		require.NotNil(t, obs.Value, "métrica %q veio nula", id)
		require.InDelta(t, want, *obs.Value, 1e-9, "métrica %q", id)
	}
}

func TestParseDetailStampsProvenance(t *testing.T) {
	for _, obs := range detail(t, "WEGE3", domain.ClassStock) {
		require.Equal(t, "WEGE3", obs.Ticker)
		require.Equal(t, "fundamentus:detalhes", obs.Source)
		require.Equal(t, domain.UnitBRL, obs.Unit)
		require.False(t, obs.FetchedAt.IsZero())
	}
}

// Página sem nenhuma das métricas e sem o papel esperado é a fonte tendo mudado
// de forma, não um ativo sem dado.
func TestParseDetailFailsOnForeignPage(t *testing.T) {
	_, err := newTickerProvider(t).Detail(t.Context(), "QUEBRA3", domain.ClassStock, false)
	require.Error(t, err)
}
