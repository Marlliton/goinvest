package b3_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func newFixtureServer(t *testing.T, fixture string) *httptest.Server {
	t.Helper()
	body, err := os.ReadFile(fixture)
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestDetailParsesUTF8Accents(t *testing.T) {
	srv := newFixtureServer(t, "testdata/get_detail_wege.json")

	detail, err := newProvider(t, srv.URL).Detail(t.Context(), "5410", false)
	require.NoError(t, err)
	require.Equal(t,
		"Bens Industriais / Máquinas e Equipamentos / Motores . Compressores e Outros",
		detail.IndustryClassification)
	require.Equal(t, "WEGE3", detail.Code)
	require.Equal(t, "5410", detail.CodeCVM)
	require.Equal(t, "84429695000111", detail.CNPJ)
	require.Len(t, detail.OtherCodes, 1)
	require.Equal(t, "BRWEGEACNOR0", detail.OtherCodes[0].ISIN)
}

func TestDetailReturnsEveryTradingCode(t *testing.T) {
	srv := newFixtureServer(t, "testdata/get_detail_itub.json")

	detail, err := newProvider(t, srv.URL).Detail(t.Context(), "19348", false)
	require.NoError(t, err)
	require.Equal(t, "Financeiro / Intermediários Financeiros / Bancos", detail.IndustryClassification)
	require.Len(t, detail.OtherCodes, 2)
	require.Equal(t, "ITUB3", detail.OtherCodes[0].Code)
	require.Equal(t, "BRITUBACNOR4", detail.OtherCodes[0].ISIN)
	require.Equal(t, "ITUB4", detail.OtherCodes[1].Code)
	require.Equal(t, "BRITUBACNPR1", detail.OtherCodes[1].ISIN)
}

func TestDetailRejectsMissingIndustryClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"code":"XPTO3","codeCVM":"1","cnpj":"0","otherCodes":[]}`)
	}))
	t.Cleanup(srv.Close)

	_, err := newProvider(t, srv.URL).Detail(t.Context(), "1", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "industryClassification")
}
