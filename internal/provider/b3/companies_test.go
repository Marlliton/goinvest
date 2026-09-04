package b3_test

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider/b3"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

func newProvider(t *testing.T, baseURL string) *b3.Provider {
	t.Helper()
	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return b3.NewProvider(client, baseURL, time.Now)
}

func decodeFilter(t *testing.T, urlPath string, into any) {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(path.Base(urlPath))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, into))
}

func newPagedServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var filter struct {
			PageNumber int `json:"pageNumber"`
		}
		decodeFilter(t, r.URL.Path, &filter)

		start := (filter.PageNumber - 1) * 2
		results := make([]string, 0, 2)
		for i := start; i < start+2 && i < 5; i++ {
			results = append(results, fmt.Sprintf(
				`{"codeCVM":"%d","issuingCompany":"EMP%d","companyName":"EMPRESA %d","cnpj":"0000000000000%d"}`,
				1000+i, i, i, i))
		}
		fmt.Fprintf(w, `{"page":{"pageNumber":%d,"pageSize":2,"totalRecords":5,"totalPages":3},"results":[%s]}`,
			filter.PageNumber, strings.Join(results, ","))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCompaniesFollowsTotalPages(t *testing.T) {
	srv, calls := newPagedServer(t)

	companies, err := newProvider(t, srv.URL).Companies(t.Context(), false)
	require.NoError(t, err)
	require.Len(t, companies, 5, "a paginação vai até totalPages, não até um número fixo")
	require.Equal(t, 3, *calls)

	require.Equal(t, "EMP0", companies[0].IssuingCompany)
	require.Equal(t, "1000", companies[0].CodeCVM)
	require.Equal(t, "EMP4", companies[4].IssuingCompany)
}

func TestCompaniesRejectsResponseWithoutTotalPages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"page":{"pageNumber":1,"pageSize":120,"totalRecords":5},"results":[]}`)
	}))
	t.Cleanup(srv.Close)

	_, err := newProvider(t, srv.URL).Companies(t.Context(), false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "totalPages")
}

func TestCompaniesReadsRealFixture(t *testing.T) {
	fixture, err := os.ReadFile("testdata/get_initial_companies_page1.json")
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var filter struct {
			PageNumber int `json:"pageNumber"`
			PageSize   int `json:"pageSize"`
		}
		decodeFilter(t, r.URL.Path, &filter)
		require.Equal(t, 120, filter.PageSize)
		// A fixture real anuncia 30 páginas; servir só a primeira e mentir o
		// totalPages mantém o teste curto sem falsear o formato do registro.
		body := strings.Replace(string(fixture), `"totalPages":30`, `"totalPages":1`, 1)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)

	companies, err := newProvider(t, srv.URL).Companies(t.Context(), false)
	require.NoError(t, err)
	require.NotEmpty(t, companies)
	require.Equal(t, "UQMU", companies[0].IssuingCompany)
	require.Equal(t, "900049", companies[0].CodeCVM)
	require.Equal(t, "46639922000144", companies[0].CNPJ)
}
