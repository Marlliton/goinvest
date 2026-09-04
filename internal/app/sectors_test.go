package app_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func seedSector(t *testing.T, db *store.DB, ticker string, class domain.AssetClass, sector, subsector string, active bool) {
	t.Helper()
	ctx := t.Context()

	require.NoError(t, db.UpsertAsset(ctx, ticker, class, ticker, collectedAt))
	a, _, err := db.GetAsset(ctx, ticker)
	require.NoError(t, err)
	require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, active, collectedAt))

	if sector != "" {
		setIdentity(t, db, ticker, sector, subsector, subsector)
	}
}

func classOf(t *testing.T, groups []app.ClassSectors, class domain.AssetClass) app.ClassSectors {
	t.Helper()
	for _, g := range groups {
		if g.Class == class {
			return g
		}
	}
	t.Fatalf("classe %s ausente na listagem", class)
	return app.ClassSectors{}
}

func TestSectorsListsByClassWithSampleMark(t *testing.T) {
	db := openTemp(t)
	for _, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedSector(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", true)
	}
	for _, ticker := range []string{"FFFF3", "GGGG3"} {
		seedSector(t, db, ticker, domain.ClassStock, "Comunicações", "Telecom", true)
	}
	seedSector(t, db, "MXRF11", domain.ClassFII, "Shoppings", "", true)

	groups, err := app.Sectors(t.Context(), db)
	require.NoError(t, err)

	stocks := classOf(t, groups, domain.ClassStock)
	require.Len(t, stocks.Groups, 2)
	require.Equal(t, "Bens Industriais", stocks.Groups[0].Name)
	require.Equal(t, 5, stocks.Groups[0].N)
	require.False(t, stocks.Groups[0].BelowThreshold)
	require.Equal(t, "Comunicações", stocks.Groups[1].Name)
	require.True(t, stocks.Groups[1].BelowThreshold, "abaixo do piso não some da lista, é marcado")

	fiis := classOf(t, groups, domain.ClassFII)
	require.Len(t, fiis.Groups, 1)
	require.True(t, fiis.Groups[0].BelowThreshold)
}

func TestSectorsExcludesInactiveFromCountButNotFromCoverage(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", true)
	seedSector(t, db, "BBBB3", domain.ClassStock, "Bens Industriais", "Máquinas", false)

	groups, err := app.Sectors(t.Context(), db)
	require.NoError(t, err)

	stocks := classOf(t, groups, domain.ClassStock)
	require.Equal(t, 1, stocks.Groups[0].N)
	require.Equal(t, 2, stocks.TotalAssets)
	require.Zero(t, stocks.IncompleteRegistry)
}

func TestSectorsCountsAssetWithoutSectorOnlyAsIncomplete(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", true)
	seedSector(t, db, "ZZZZ3", domain.ClassStock, "", "", true)

	groups, err := app.Sectors(t.Context(), db)
	require.NoError(t, err)

	stocks := classOf(t, groups, domain.ClassStock)
	require.Len(t, stocks.Groups, 1, "sem setor não vira linha da listagem")
	require.Equal(t, 1, stocks.Groups[0].N)
	require.Equal(t, 1, stocks.IncompleteRegistry)
	require.Equal(t, 2, stocks.TotalAssets)
}

func TestSectorsDescend(t *testing.T) {
	db := openTemp(t)
	for _, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedSector(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", true)
	}
	seedSector(t, db, "FFFF3", domain.ClassStock, "Bens Industriais", "Transporte", true)

	subs, err := app.SectorsDescend(t.Context(), db, "Bens Industriais")
	require.NoError(t, err)
	require.Len(t, subs, 2)
	require.Equal(t, "Máquinas", subs[0].Name)
	require.Equal(t, 5, subs[0].N)
	require.False(t, subs[0].BelowThreshold)
	require.True(t, subs[1].BelowThreshold)
}

func TestSectorsDescendUnknownSector(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", true)

	_, err := app.SectorsDescend(t.Context(), db, "Setor Inexistente")
	require.ErrorIs(t, err, app.ErrSectorNotFound)
}
