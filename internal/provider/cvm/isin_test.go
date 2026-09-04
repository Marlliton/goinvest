package cvm_test

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/cvm"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

func zipWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	require.NoError(t, err)
	_, err = f.Write(content)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

func newServer(t *testing.T, byYear map[int][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for year, body := range byYear {
			if strings.HasSuffix(r.URL.Path, fmt.Sprintf("inf_mensal_fii_%d.zip", year)) {
				w.Write(body)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newProvider(t *testing.T, baseURL string, years ...int) *cvm.Provider {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return cvm.NewProvider(client, baseURL, years, time.Now)
}

func realFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/inf_mensal_fii_geral_sample.csv")
	require.NoError(t, err)
	return body
}

func TestISINByCNPJDecodesISO88591CSVFromZip(t *testing.T) {
	srv := newServer(t, map[int][]byte{
		2026: zipWith(t, "inf_mensal_fii_geral_2026.csv", realFixture(t)),
	})

	byCNPJ, err := newProvider(t, srv.URL, 2026).ISINByCNPJ(t.Context(), false)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"00332266000131": "BRFVPQCTF015"}, byCNPJ,
		"CNPJ vira só dígitos; o ISIN vai como veio")
}

func TestISINByCNPJPrefersMostRecentReference(t *testing.T) {
	header := strings.SplitN(string(realFixture(t)), "\n", 2)[0]
	old := header + "\n" + strings.Join([]string{
		"Classe", "00.332.266/0001-31", "2024-01-01", "1", "2024-02-10", "VIA PARQUE",
		"1994-11-24", "INVESTIDORES EM GERAL", "BROLDACTF015",
	}, ";") + "\n"

	srv := newServer(t, map[int][]byte{
		2026: zipWith(t, "inf_mensal_fii_geral_2026.csv", realFixture(t)),
		2024: zipWith(t, "inf_mensal_fii_geral_2024.csv", []byte(old)),
	})

	// O ano antigo é lido por último de propósito.
	byCNPJ, err := newProvider(t, srv.URL, 2026, 2024).ISINByCNPJ(t.Context(), false)
	require.NoError(t, err)
	require.Equal(t, "BRFVPQCTF015", byCNPJ["00332266000131"])

	byCNPJ, err = newProvider(t, srv.URL, 2024, 2026).ISINByCNPJ(t.Context(), false)
	require.NoError(t, err)
	require.Equal(t, "BRFVPQCTF015", byCNPJ["00332266000131"], "a ordem dos anos não decide")
}

func TestISINByCNPJRejectsZipWithoutExpectedEntry(t *testing.T) {
	srv := newServer(t, map[int][]byte{
		2026: zipWith(t, "outro_arquivo.csv", realFixture(t)),
	})

	_, err := newProvider(t, srv.URL, 2026).ISINByCNPJ(t.Context(), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inf_mensal_fii_geral_2026.csv")
}

func TestISINByCNPJRejectsLegacyZipEntryName(t *testing.T) {
	srv := newServer(t, map[int][]byte{
		2026: zipWith(t, "geral_2026.csv", realFixture(t)),
	})

	_, err := newProvider(t, srv.URL, 2026).ISINByCNPJ(t.Context(), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "inf_mensal_fii_geral_2026.csv")
}

func TestISINByCNPJRejectsMissingColumn(t *testing.T) {
	srv := newServer(t, map[int][]byte{
		2026: zipWith(t, "inf_mensal_fii_geral_2026.csv", []byte("Tipo_Fundo_Classe;CNPJ_Fundo_Classe\nClasse;00.332.266/0001-31\n")),
	})

	_, err := newProvider(t, srv.URL, 2026).ISINByCNPJ(t.Context(), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Codigo_ISIN")
}

func TestISINByCNPJSkipsRowsWithoutISIN(t *testing.T) {
	header := strings.SplitN(string(realFixture(t)), "\n", 2)[0]
	body := header + "\n" + strings.Join([]string{
		"Classe", "11.111.111/0001-11", "2026-01-01", "1", "2026-02-10", "SEM ISIN",
		"1994-11-24", "INVESTIDORES EM GERAL", "",
	}, ";") + "\n"

	srv := newServer(t, map[int][]byte{2026: zipWith(t, "inf_mensal_fii_geral_2026.csv", []byte(body))})

	byCNPJ, err := newProvider(t, srv.URL, 2026).ISINByCNPJ(t.Context(), false)
	require.NoError(t, err)
	require.Empty(t, byCNPJ)
}
