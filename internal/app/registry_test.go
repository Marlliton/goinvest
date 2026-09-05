package app_test

import (
	"context"
	"errors"
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/stretchr/testify/require"
)

type singleCompanyIdentity struct{}

func (singleCompanyIdentity) Name() string { return "fake-identity-single" }

func (singleCompanyIdentity) Companies(context.Context, bool) ([]identity.CompanyRef, error) {
	return []identity.CompanyRef{{IssuingCompany: "WEGE", CodeCVM: "5410"}}, nil
}

func (singleCompanyIdentity) Detail(context.Context, string, bool) (identity.CompanyDetail, error) {
	return identity.CompanyDetail{
		Code: "WEGE3", CodeCVM: "5410", CNPJ: "84429695000111",
		IndustryClassification: "Bens Industriais / Máquinas e Equipamentos / Motores . Compressores e Outros",
		OtherCodes:             []identity.CompanyCode{{Code: "WEGE3", ISIN: "BRWEGEACNOR0"}},
	}, nil
}

type noFIIIdentity struct{}

func (noFIIIdentity) Name() string { return "fake-fii-none" }

func (noFIIIdentity) ISINByCNPJ(context.Context, bool) (map[string]string, error) {
	return map[string]string{}, nil
}

func (noFIIIdentity) Segments(context.Context, bool) (map[string]string, error) {
	return map[string]string{}, nil
}

type failingFIIIdentity struct{ err error }

func (f failingFIIIdentity) Name() string { return "fake-fii-failing" }

func (f failingFIIIdentity) ISINByCNPJ(context.Context, bool) (map[string]string, error) {
	return nil, f.err
}

func (failingFIIIdentity) Segments(context.Context, bool) (map[string]string, error) {
	return map[string]string{}, nil
}

func TestRegistryAllRecomputesSectorReferenceWithoutGoingThroughCLI(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()

	require.NoError(t, db.UpsertAsset(ctx, "WEGE3", domain.ClassStock, "", collectedAt))
	a, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, collectedAt))

	_, err = app.RegistryAll(ctx, app.RegistryAllConfig{
		Stocks:  app.RegistryConfig{DB: db, Identity: singleCompanyIdentity{}, Now: now},
		FIIs:    app.RegistryFIIConfig{DB: db, CVM: noFIIIdentity{}, Fundamentus: noFIIIdentity{}, Now: now},
		Catalog: loadCatalog(t),
		Now:     now,
	})
	require.NoError(t, err)

	updated, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.NotEmpty(t, updated.PeerGroupLevel)
}

func TestRegistryAllRecomputesAndStillReturnsErrorWhenFIIStageFailsAfterStocksWroteIdentity(t *testing.T) {
	db := openTemp(t)
	ctx := t.Context()

	require.NoError(t, db.UpsertAsset(ctx, "WEGE3", domain.ClassStock, "", collectedAt))
	a, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, collectedAt))

	wantErr := errors.New("cvm indisponível")
	_, err = app.RegistryAll(ctx, app.RegistryAllConfig{
		Stocks:  app.RegistryConfig{DB: db, Identity: singleCompanyIdentity{}, Now: now},
		FIIs:    app.RegistryFIIConfig{DB: db, CVM: failingFIIIdentity{err: wantErr}, Fundamentus: noFIIIdentity{}, Now: now},
		Catalog: loadCatalog(t),
		Now:     now,
	})
	require.ErrorIs(t, err, wantErr)

	updated, _, err := db.GetAsset(ctx, "WEGE3")
	require.NoError(t, err)
	require.NotEmpty(t, updated.PeerGroupLevel)
}
