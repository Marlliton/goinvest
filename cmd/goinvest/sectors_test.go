package main

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func sectorDeps(t *testing.T) rootDeps {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "goinvest.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	ctx := t.Context()
	seed := func(ticker string, class domain.AssetClass, sector, subsector string) {
		require.NoError(t, db.UpsertAsset(ctx, ticker, class, ticker, collectedAt))
		a, _, err := db.GetAsset(ctx, ticker)
		require.NoError(t, err)
		require.NoError(t, db.UpdateAssetLiquidity(ctx, a.AssetID, true, collectedAt))
		require.NoError(t, db.UpdateAssetIdentities(ctx, []store.AssetIdentityUpdate{{
			AssetID: a.AssetID, Sector: sector, Subsector: subsector, Segment: subsector,
			SectorSrc: "b3", UpdatedAt: collectedAt,
		}}))
	}
	for _, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seed(ticker, domain.ClassStock, "Bens Industriais", "Máquinas")
	}
	seed("MXRF11", domain.ClassFII, "Shoppings", "")

	return rootDeps{DB: db}
}

func TestSectorsCommand(t *testing.T) {
	var out bytes.Buffer

	cmd := newSectorsCmd(sectorDeps(t))
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	text := out.String()
	require.Contains(t, text, "Ações")
	require.Contains(t, text, "FIIs")
	require.Contains(t, text, "Bens Industriais")
	require.Contains(t, text, "Shoppings")
	require.Contains(t, text, "referência de mercado", "o setor com 1 FII é marcado, não some")
}

func TestSectorsCommandDescend(t *testing.T) {
	var out bytes.Buffer

	cmd := newSectorsCmd(sectorDeps(t))
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"Bens Industriais"})
	require.NoError(t, cmd.ExecuteContext(t.Context()))

	require.Contains(t, out.String(), "Máquinas")
}

func TestSectorsCommandDescendUnknownSector(t *testing.T) {
	cmd := newSectorsCmd(sectorDeps(t))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"Setor Inexistente"})

	err := cmd.ExecuteContext(t.Context())
	require.Error(t, err)
	require.Contains(t, err.Error(), "Setor Inexistente")
	require.Contains(t, err.Error(), "goinvest sectors")
	require.NotContains(t, err.Error(), "sql")
	require.NotContains(t, err.Error(), "ErrNoRows")
}
