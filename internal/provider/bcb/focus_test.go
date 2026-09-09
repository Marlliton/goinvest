package bcb_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/bcb"
	"github.com/stretchr/testify/require"
)

func newFocusProvider(t *testing.T, baseURL string) *bcb.FocusProvider {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return bcb.NewFocusProvider(client, baseURL, time.Now)
}

func newFocusBodyServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFocusIPCA12mRequestsOnlySimpleFilter(t *testing.T) {
	body, err := os.ReadFile("testdata/focus_ipca12m.json")
	require.NoError(t, err)

	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Write(body)
	}))
	t.Cleanup(srv.Close)

	_, _, err = newFocusProvider(t, srv.URL).FocusIPCA12m(t.Context(), false)
	require.NoError(t, err)
	require.Equal(t, "/olinda/servico/Expectativas/versao/v1/odata/ExpectativasMercadoInflacao12Meses", gotPath)
	require.Contains(t, gotQuery, "Indicador")
	require.NotContains(t, gotQuery, "and", "filtro composto é frágil na Focus")
}

func TestFocusIPCA12mFiltersSuavizadaAndBaseCalculo(t *testing.T) {
	body, err := os.ReadFile("testdata/focus_ipca12m.json")
	require.NoError(t, err)
	srv := newFocusBodyServer(t, body)

	rate, referenceAt, err := newFocusProvider(t, srv.URL).FocusIPCA12m(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 0.042891, rate, 1e-9, "percentual é fração em todo o pipeline")
	require.Equal(t, time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC), referenceAt)
}

func TestFocusIPCA12mNoPointsAfterFilterIsError(t *testing.T) {
	srv := newFocusBodyServer(t, []byte(`{"value":[{"Indicador":"IPCA","Data":"2026-09-01","Mediana":4.5,"Suavizada":"N","baseCalculo":1}]}`))

	_, _, err := newFocusProvider(t, srv.URL).FocusIPCA12m(t.Context(), false)
	require.Error(t, err)
}

func TestFocusIPCA12mMalformedJSONIsError(t *testing.T) {
	srv := newFocusBodyServer(t, []byte(`<html>manutenção</html>`))

	_, _, err := newFocusProvider(t, srv.URL).FocusIPCA12m(t.Context(), false)
	require.Error(t, err)
}
