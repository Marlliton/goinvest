package app_test

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

// A janela fechada com fixedNow (03/09/2026) é 2021..2025.
func seedBazinYears(t *testing.T, db *store.DB, ticker string, first, last int, perYear float64) {
	t.Helper()
	var events []domain.DividendEvent
	for year := first; year <= last; year++ {
		events = append(events,
			event(exOn(year, time.March, 10), domain.DividendCash, perYear/2, 1),
			event(exOn(year, time.September, 10), domain.DividendCash, perYear/2, 1))
	}
	seedDividends(t, db, ticker, events...)
}

func bazinReport(t *testing.T, db *store.DB, ticker string) app.Report {
	t.Helper()
	report, err := app.Show(t.Context(), db, loadCatalog(t), ticker, defaultTax, now)
	require.NoError(t, err)
	require.NotNil(t, report.Bazin)
	return report
}

func seedBBAS3(t *testing.T, db *store.DB, withSelic bool) *store.DB {
	t.Helper()
	values := wege3Values()
	values["dy"] = ptr(0.09)
	values["cotacao"] = ptr(24.00)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	if withSelic {
		require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	}
	return db
}

func TestShowGoldenOutputRange(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["cotacao"] = ptr(24.00)
	values["dy"] = ptr(0.02)
	values["cresc_rec_5a"] = ptr(0.06)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	text := app.RenderText(report)
	requireGolden(t, "show_bbas3.txt", text)

	require.Contains(t, text, "Faixa:", "a faixa Bazin-Gordon aparece na tela")
	require.Contains(t, text, "Gordon: Ke", "o retorno exigido fica visível")
	require.Contains(t, text, "sensibilidade", "a tabela de sensibilidade aparece")
	require.GreaterOrEqual(t, len(report.Bazin.Gordon.Sensitivity), 3,
		"o critério da fase pede pelo menos três hipóteses de crescimento")

	growths := map[float64]bool{}
	for _, point := range report.Bazin.Gordon.Sensitivity {
		growths[point.Growth] = true
	}
	require.GreaterOrEqual(t, len(growths), 3, "as hipóteses precisam ser distintas entre si")
	require.Positive(t, report.Bazin.Gordon.ImpliedGrowth,
		"payout abaixo de 100% precisa produzir crescimento implícito positivo")
}

func TestShow_BazinCeiling(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	// 1,20 por ano em dividendo puro: teto = 1,20 / 0,06 = 20,00.
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.Empty(t, report.Bazin.NotApplicableReason)
	require.InDelta(t, 20.00, report.Bazin.Ceiling, 1e-9)
	require.InDelta(t, 24.00, report.Bazin.CurrentPrice, 1e-9)
	require.InDelta(t, 0.20, report.Bazin.PremiumDiscount, 1e-9)
	require.Equal(t, 5, report.Bazin.YearsUsed)
	require.Len(t, report.Bazin.Years, 5)

	text := app.RenderText(report)
	require.Contains(t, text, "Preço-teto (Bazin)")
	require.Contains(t, text, "Teto: R$ 20,00")
	require.Contains(t, text, "Ágio/deságio: +20,00%")
	require.Contains(t, text, "Anos usados: 5 de 5")
	require.Contains(t, text, "2025: R$ 1,20")
	require.Contains(t, text, "DY−Selic", "o teto nunca sai sozinho")
}

func TestShow_BazinDiscount(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.80)

	report := bazinReport(t, db, "BBAS3")
	require.InDelta(t, 30.00, report.Bazin.Ceiling, 1e-9)
	require.InDelta(t, -0.20, report.Bazin.PremiumDiscount, 1e-9)
	require.Contains(t, app.RenderText(report), "Ágio/deságio: -20,00%")
}

func TestShow_BazinFloorNotMet(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2024, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.NotEmpty(t, report.Bazin.NotApplicableReason)
	require.Zero(t, report.Bazin.Ceiling)
	require.Contains(t, report.Bazin.NotApplicableReason, "3")

	text := app.RenderText(report)
	require.Contains(t, text, "Preço-teto (Bazin): não aplicável")
	require.NotContains(t, text, "Teto: R$")
}

func TestShow_BazinRequiresSelic(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), false)
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.Contains(t, report.Bazin.NotApplicableReason, "Selic")
	require.Zero(t, report.Bazin.Ceiling)
	require.NotContains(t, app.RenderText(report), "Teto: R$")
}

func TestShow_BazinRequiresDividendYield(t *testing.T) {
	values := wege3Values()
	values["dy"] = nil
	values["cotacao"] = ptr(24.00)
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.Contains(t, report.Bazin.NotApplicableReason, "DY")
	require.Zero(t, report.Bazin.Ceiling)
}

func TestShow_BazinListedLessThan5Years(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2023, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.Empty(t, report.Bazin.NotApplicableReason)
	require.Equal(t, 3, report.Bazin.YearsUsed)
	require.InDelta(t, 20.00, report.Bazin.Ceiling, 1e-9)
	require.Contains(t, app.RenderText(report), "Anos usados: 3 de 5")
}

func TestShow_BazinMissingYearInWindow(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2021, 2022, 1.20)
	seedBazinYears(t, db, "BBAS3", 2024, 2025, 1.20)

	report := bazinReport(t, db, "BBAS3")
	require.Contains(t, report.Bazin.NotApplicableReason, "2023")
	require.Zero(t, report.Bazin.Ceiling)
}

func TestShow_BazinWithoutDividends(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)

	report := bazinReport(t, db, "BBAS3")
	require.Contains(t, report.Bazin.NotApplicableReason, "detalhar BBAS3")
	require.Zero(t, report.Bazin.Ceiling)
}

func TestShow_BazinAtypicalYearIsVisible(t *testing.T) {
	db := seedBBAS3(t, openTemp(t), true)
	seedBazinYears(t, db, "BBAS3", 2021, 2024, 1.20)
	seedDividends(t, db, "BBAS3",
		event(exOn(2025, time.March, 10), domain.DividendCash, 1.10, 1),
		event(exOn(2025, time.September, 10), domain.DividendCash, 0.10, 1))

	report := bazinReport(t, db, "BBAS3")
	require.Equal(t, []int{2025}, report.Bazin.AtypicalYears)
	require.Contains(t, app.RenderText(report), "· ano 2025 concentrado")
}

func TestShow_BazinFIIWithoutDividends(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87), "pvp": ptr(1.02), "dy": ptr(0.132),
	})
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))

	report := bazinReport(t, db, "MXRF11")
	require.NotEmpty(t, report.Bazin.NotApplicableReason)
	require.Zero(t, report.Bazin.Ceiling)
}
