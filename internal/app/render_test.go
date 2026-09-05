package app_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestRenderSectorsDescendFallsBackToSectorWhenSectorItselfIsAboveThreshold(t *testing.T) {
	db := openTemp(t)
	for _, ticker := range []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3"} {
		seedSector(t, db, ticker, domain.ClassStock, "Bens Industriais", "Máquinas", true)
	}
	seedSector(t, db, "FFFF3", domain.ClassStock, "Bens Industriais", "Transporte", true)

	descend, err := app.SectorsDescend(t.Context(), db, "Bens Industriais")
	require.NoError(t, err)

	text := app.RenderSectorsDescend("Bens Industriais", descend)
	require.Contains(t, text, "referência do setor")
	require.NotContains(t, text, "referência de mercado")
}

func TestRenderSectorsDescendFallsBackToMarketWhenSectorItselfIsBelowThreshold(t *testing.T) {
	db := openTemp(t)
	for _, ticker := range []string{"AAAA3", "BBBB3", "CCCC3"} {
		seedSector(t, db, ticker, domain.ClassStock, "Comunicações", "Telecom", true)
	}
	seedSector(t, db, "DDDD3", domain.ClassStock, "Comunicações", "Mídia", true)

	descend, err := app.SectorsDescend(t.Context(), db, "Comunicações")
	require.NoError(t, err)

	text := app.RenderSectorsDescend("Comunicações", descend)
	require.Contains(t, text, "referência de mercado")
	require.NotContains(t, text, "referência do setor")
}

func TestRenderSectorsStillFallsBackToMarketForTopLevelSector(t *testing.T) {
	groups := []app.ClassSectors{{
		Class: domain.ClassStock,
		Groups: []app.SectorGroup{
			{Name: "Comunicações", N: 4, BelowThreshold: true},
		},
	}}

	text := app.RenderSectors(groups)
	require.Contains(t, text, "referência de mercado")
}
