package app_test

import (
	"strings"
	"testing"
	"time"

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

func TestRenderSectorsDescendFIISingleLevelExplainsAbsenceOfSubsector(t *testing.T) {
	text := app.RenderSectorsDescend("Logística", app.SectorDescend{SingleLevel: true, N: 16})
	require.Contains(t, text, "nível só")
	require.Contains(t, text, "16")
	require.NotContains(t, text, "subsetores")
}

func TestRenderSectorsDescendNotesFIICollisionForStockSector(t *testing.T) {
	text := app.RenderSectorsDescend("Outros", app.SectorDescend{
		Groups:  []app.SectorGroup{{Name: "Diversos", N: 5}},
		AlsoFII: true,
	})
	require.Contains(t, text, "FII")
}

func peerN(n int) *int { return &n }

func TestRenderTextExplainsSmallerPeerN(t *testing.T) {
	report := app.Report{
		Ticker: "WEGE3",
		Class:  domain.ClassStock,
		Header: app.HeaderView{PeerGroupLabel: "Bens Industriais", PeerGroupN: 25},
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{{
				Label:      "P/L",
				Value:      ptr(34.94),
				Percentile: ptr(0.6),
				PeerN:      peerN(18),
			}},
		}},
	}

	text := app.RenderText(report)
	require.Contains(t, text, "n=18")
	require.Contains(t, text, "quantos papéis")
}

func TestRenderTextOmitsPeerNExplanationWhenLineNMatchesHeader(t *testing.T) {
	report := app.Report{
		Ticker: "WEGE3",
		Class:  domain.ClassStock,
		Header: app.HeaderView{PeerGroupLabel: "Bens Industriais", PeerGroupN: 25},
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{{
				Label:      "P/L",
				Value:      ptr(34.94),
				Percentile: ptr(0.6),
				PeerN:      peerN(25),
			}},
		}},
	}

	text := app.RenderText(report)
	require.NotContains(t, text, "quantos papéis")
}

func refAt(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

func TestRenderText_NotApplicable(t *testing.T) {
	report := app.Report{
		Ticker: "ITUB4",
		Class:  domain.ClassStock,
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{{
				Label:               "EV/EBITDA",
				NotApplicableReason: "Banco: dívida é matéria-prima",
			}},
		}},
	}

	text := app.RenderText(report)
	require.Contains(t, text, "Banco: dívida é matéria-prima")
	require.NotContains(t, text, "EV/EBITDA: —", "não aplicável não é a mesma coisa que ausente")
	require.Contains(t, text, "não se aplica", "a legenda ganha o quarto estado")
}

func TestRenderText_NotApplicableLegendIsAbsentWhenUnused(t *testing.T) {
	report := app.Report{
		Ticker: "WEGE3",
		Class:  domain.ClassStock,
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{{Label: "P/L", Value: ptr(30.0)}},
		}},
	}

	require.NotContains(t, app.RenderText(report), "não se aplica")
}

func TestRenderText_ReferenceAtDivergent(t *testing.T) {
	report := app.Report{
		Ticker: "WEGE3",
		Class:  domain.ClassStock,
		Header: app.HeaderView{ReferenceAt: refAt(2026, time.August, 15)},
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{
				{Label: "P/L", Value: ptr(30.0), ReferenceAt: refAt(2026, time.August, 15)},
				{Label: "EBIT (12m)", Value: ptr(1e9), Unit: domain.UnitBRL, ReferenceAt: refAt(2026, time.July, 15)},
			},
		}},
	}

	lines := strings.Split(app.RenderText(report), "\n")
	require.NotContains(t, lineWith(t, lines, "P/L"), "ref ")
	require.Contains(t, lineWith(t, lines, "EBIT (12m)"), "· ref 15/07")
}

func TestRenderText_ReferenceAtNilNeverMarks(t *testing.T) {
	report := app.Report{
		Ticker: "WEGE3",
		Class:  domain.ClassStock,
		Header: app.HeaderView{ReferenceAt: refAt(2026, time.August, 15)},
		Blocks: []app.BlockView{{
			Label: "Valuation",
			Lines: []app.LineView{{Label: "P/L", Value: ptr(30.0)}},
		}},
	}

	require.NotContains(t, app.RenderText(report), "ref ")
}

func lineWith(t *testing.T, lines []string, label string) string {
	t.Helper()
	for _, l := range lines {
		if strings.Contains(l, label+":") {
			return l
		}
	}
	t.Fatalf("linha %q não está na saída", label)
	return ""
}
