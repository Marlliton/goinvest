package bcb_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCDIBodyServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/dados/serie/bcdata.sgs.4389/dados/ultimos/5", r.URL.Path)
		assert.Equal(t, "formato=json", r.URL.RawQuery)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newCDIFixtureServer(t *testing.T, fixture string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(fixture)
	require.NoError(t, err)
	return newCDIBodyServer(t, body)
}

func TestCDIReturnsMostRecentPoint(t *testing.T) {
	srv := newCDIFixtureServer(t, "testdata/cdi_4389.json")

	value, referenceAt, err := newProvider(t, srv.URL).CDI(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 0.1465, value, 1e-9, "percentual é fração em todo o pipeline")
	require.Equal(t, time.Date(2026, 9, 16, 0, 0, 0, 0, time.UTC), referenceAt)
}

func TestCDIEmptySeriesIsError(t *testing.T) {
	srv := newCDIBodyServer(t, []byte(`[]`))

	_, _, err := newProvider(t, srv.URL).CDI(t.Context(), false)
	require.Error(t, err)
}

func TestCDIMalformedValueIsError(t *testing.T) {
	srv := newCDIBodyServer(t, []byte(`[{"data":"16/09/2026","valor":"14,65"}]`))

	_, _, err := newProvider(t, srv.URL).CDI(t.Context(), false)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "14,65"))
}

func TestCDIMalformedJSONIsError(t *testing.T) {
	srv := newCDIBodyServer(t, []byte(`<html>manutenção</html>`))

	_, _, err := newProvider(t, srv.URL).CDI(t.Context(), false)
	require.Error(t, err)
}
