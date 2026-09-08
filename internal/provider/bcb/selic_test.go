package bcb_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/bcb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

func newProvider(t *testing.T, baseURL string) *bcb.Provider {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return bcb.NewProvider(client, baseURL, time.Now)
}

func newBodyServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dados/serie/bcdata.sgs.432/dados/ultimos/5", r.URL.Path)
		assert.Equal(t, "formato=json", r.URL.RawQuery)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newFixtureServer(t *testing.T, fixture string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(fixture)
	require.NoError(t, err)
	return newBodyServer(t, body)
}

func TestSelicReturnsMostRecentPoint(t *testing.T) {
	srv := newFixtureServer(t, "testdata/selic_432.json")

	value, referenceAt, err := newProvider(t, srv.URL).Selic(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 14.00, value, 1e-9)
	require.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), referenceAt)
}

func TestSelicEmptySeriesIsError(t *testing.T) {
	srv := newBodyServer(t, []byte(`[]`))

	_, _, err := newProvider(t, srv.URL).Selic(t.Context(), false)
	require.Error(t, err)
}

func TestSelicMalformedValueIsError(t *testing.T) {
	srv := newBodyServer(t, []byte(`[{"data":"16/09/2026","valor":"14,00"}]`))

	_, _, err := newProvider(t, srv.URL).Selic(t.Context(), false)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "14,00"))
}

func TestSelicMalformedJSONIsError(t *testing.T) {
	srv := newBodyServer(t, []byte(`<html>manutenção</html>`))

	_, _, err := newProvider(t, srv.URL).Selic(t.Context(), false)
	require.Error(t, err)
}
