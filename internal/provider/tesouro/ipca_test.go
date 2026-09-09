package tesouro_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/tesouro"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

var fixedNow = func() time.Time { return time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC) }

func newProvider(t *testing.T, baseURL string, now func() time.Time) *tesouro.Provider {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return tesouro.NewProvider(client, baseURL, now)
}

func newBodyServer(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestIPCA10yChoosesExactTypeAndClosestMaturityFromLatestBaseDate(t *testing.T) {
	srv := newFixtureServer(t, "testdata/tesouro_ipca_sample.csv")

	rate, referenceAt, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 0.0766, rate, 1e-9, "vencimento 15/08/2036 é o mais próximo de 10 anos a partir de 08/09/2026")
	require.Equal(t, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), referenceAt)
}

func TestIPCA10yIgnoresSemiAnnualCouponVariant(t *testing.T) {
	body := []byte(
		"Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n" +
			"Tesouro IPCA+ com Juros Semestrais;15/08/2036;08/09/2026;1,00;1,02;1000,00;999,00;999,50\n" +
			"Tesouro IPCA+;15/01/2030;08/09/2026;6,50;6,52;1000,00;999,00;999,50\n",
	)
	srv := newBodyServer(t, body)

	rate, _, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 0.065, rate, 1e-9, "a variante com cupom semestral não pode vencer mesmo tendo vencimento mais próximo")
}

func TestIPCA10yIgnoresOlderBaseDate(t *testing.T) {
	body := []byte(
		"Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n" +
			"Tesouro IPCA+;08/09/2036;01/01/2020;9,99;9,99;1000,00;999,00;999,50\n" +
			"Tesouro IPCA+;15/01/2030;08/09/2026;6,50;6,52;1000,00;999,00;999,50\n",
	)
	srv := newBodyServer(t, body)

	rate, referenceAt, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.NoError(t, err)
	require.InDelta(t, 0.065, rate, 1e-9, "a linha de Data Base antiga não pode vencer mesmo tendo vencimento mais próximo")
	require.Equal(t, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC), referenceAt)
}

func TestIPCA10yNoReferenceTypeIsError(t *testing.T) {
	body := []byte(
		"Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n" +
			"Tesouro IPCA+ com Juros Semestrais;15/08/2036;08/09/2026;1,00;1,02;1000,00;999,00;999,50\n",
	)
	srv := newBodyServer(t, body)

	_, _, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.Error(t, err)
}

func TestIPCA10yEmptyCSVIsError(t *testing.T) {
	body := []byte("Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n")
	srv := newBodyServer(t, body)

	_, _, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.Error(t, err)
}

func TestIPCA10yMalformedRateIsError(t *testing.T) {
	body := []byte(
		"Tipo Titulo;Data Vencimento;Data Base;Taxa Compra Manha;Taxa Venda Manha;PU Compra Manha;PU Venda Manha;PU Base Manha\n" +
			"Tesouro IPCA+;15/08/2036;08/09/2026;abc;6,52;1000,00;999,00;999,50\n",
	)
	srv := newBodyServer(t, body)

	_, _, err := newProvider(t, srv.URL, fixedNow).IPCA10y(t.Context(), false)
	require.Error(t, err)
}
