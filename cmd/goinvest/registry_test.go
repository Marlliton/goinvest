package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

var collectedAt = time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)

type fakeIdentity struct{}

func (fakeIdentity) Name() string { return "fake" }

func (fakeIdentity) Companies(context.Context, bool) ([]identity.CompanyRef, error) {
	return []identity.CompanyRef{
		{IssuingCompany: "WEGE", CodeCVM: "5410"},
		{IssuingCompany: "ITUB", CodeCVM: "19348"},
	}, nil
}

func (fakeIdentity) Detail(_ context.Context, codeCVM string, _ bool) (identity.CompanyDetail, error) {
	switch codeCVM {
	case "5410":
		return identity.CompanyDetail{
			Code: "WEGE3", CodeCVM: "5410", CNPJ: "84429695000111",
			IndustryClassification: "Bens Industriais / Máquinas e Equipamentos / Motores . Compressores e Outros",
			OtherCodes:             []identity.CompanyCode{{Code: "WEGE3", ISIN: "BRWEGEACNOR0"}},
		}, nil
	default:
		return identity.CompanyDetail{
			Code: "ITUB4", CodeCVM: "19348", CNPJ: "60872504000123",
			IndustryClassification: "Financeiro / Intermediários Financeiros / Bancos",
			OtherCodes:             []identity.CompanyCode{{Code: "ITUB4", ISIN: "BRITUBACNPR1"}},
		}, nil
	}
}

type fakeFII struct{}

func (fakeFII) Name() string { return "fake-fii" }

func (fakeFII) ISINByCNPJ(context.Context, bool) (map[string]string, error) {
	return map[string]string{"00332266000131": "BRFVPQCTF015"}, nil
}

func (fakeFII) Segments(context.Context, bool) (map[string]string, error) {
	return map[string]string{"FVPQ11": "Shoppings"}, nil
}

func testDeps(t *testing.T) rootDeps {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	ctx := t.Context()
	for _, ticker := range []string{"WEGE3", "ITUB4"} {
		require.NoError(t, db.UpsertAsset(ctx, ticker, domain.ClassStock, "", collectedAt))
		a, _, err := db.GetAsset(ctx, ticker)
		require.NoError(t, err)
		require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, collectedAt))
	}

	require.NoError(t, db.UpsertAsset(ctx, "FVPQ11", domain.ClassFII, "", collectedAt))

	return rootDeps{DB: db, B3: fakeIdentity{}, CVM: fakeFII{}, Fundamentus: fakeFII{}}
}

func TestRegistryCmdPrintsOneLinePerProgressOutsideTTY(t *testing.T) {
	var out bytes.Buffer

	cmd := newRegistryCmd(testDeps(t))
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	text := out.String()
	require.NotContains(t, text, "\r", "fora de TTY nada é reescrito")

	progress := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "cadastro:") {
			progress++
		}
	}
	require.GreaterOrEqual(t, progress, 1, "cada atualização de progresso é uma linha")
	require.Contains(t, text, "2 de 2")
}

func TestRegistryCmdCoversStocksAndFIIs(t *testing.T) {
	var out bytes.Buffer
	deps := testDeps(t)

	cmd := newRegistryCmd(deps)
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(nil)
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Contains(t, out.String(), "ações")
	require.Contains(t, out.String(), "FIIs")

	a, _, err := deps.DB.GetAsset(t.Context(), "FVPQ11")
	require.NoError(t, err)
	require.Equal(t, "Shoppings", a.Sector)
}
