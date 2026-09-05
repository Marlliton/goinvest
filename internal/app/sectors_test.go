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

func TestSectorsCoverageExcludesInactiveAssets(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", true)
	seedSector(t, db, "BBBB3", domain.ClassStock, "Bens Industriais", "Máquinas", false)

	groups, err := app.Sectors(t.Context(), db)
	require.NoError(t, err)

	stocks := classOf(t, groups, domain.ClassStock)
	require.Equal(t, 1, stocks.Groups[0].N)
	require.Equal(t, 1, stocks.TotalAssets, "só a ação ativa entra na conta")
	require.Zero(t, stocks.IncompleteRegistry)
}

func TestSectorsFIILiquidAssetWithoutSectorCountsAsIncomplete(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "MXRF11", domain.ClassFII, "Shoppings", "", true)
	seedSector(t, db, "SEMS11", domain.ClassFII, "", "", true)
	seedSector(t, db, "DEAD11", domain.ClassFII, "", "", false)

	groups, err := app.Sectors(t.Context(), db)
	require.NoError(t, err)

	fiis := classOf(t, groups, domain.ClassFII)
	require.Equal(t, 2, fiis.TotalAssets, "só os dois ativos entram na conta")
	require.Equal(t, 1, fiis.IncompleteRegistry)
	require.Len(t, fiis.Groups, 1)
	require.Equal(t, "Shoppings", fiis.Groups[0].Name)
	require.Equal(t, 1, fiis.Groups[0].N)
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

	descend, err := app.SectorsDescend(t.Context(), db, "Bens Industriais")
	require.NoError(t, err)
	require.False(t, descend.BelowThreshold, "o setor tem 6 ativos líquidos no total, acima do piso")
	require.Len(t, descend.Groups, 2)
	require.Equal(t, "Máquinas", descend.Groups[0].Name)
	require.Equal(t, 5, descend.Groups[0].N)
	require.False(t, descend.Groups[0].BelowThreshold)
	require.True(t, descend.Groups[1].BelowThreshold)
}

func TestSectorsDescendUnknownSector(t *testing.T) {
	db := openTemp(t)
	seedSector(t, db, "AAAA3", domain.ClassStock, "Bens Industriais", "Máquinas", true)

	_, err := app.SectorsDescend(t.Context(), db, "Setor Inexistente")
	require.ErrorIs(t, err, app.ErrSectorNotFound)
}
