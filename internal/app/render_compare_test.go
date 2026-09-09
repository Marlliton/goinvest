package app_test

import (
	"strings"
	"testing"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func compareOf(t *testing.T, db *store.DB, tickers ...string) app.CompareReport {
	t.Helper()
	report, err := app.Compare(t.Context(), db, loadCatalog(t), tickers, now)
	require.NoError(t, err)
	return report
}

func TestRenderCompareText_TwoTables(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3")
	seed(t, db, "MXRF11", domain.ClassFII, map[domain.MetricID]*float64{
		"cotacao": ptr(9.87), "pvp": ptr(1.02), "dy": ptr(0.132), "vacancia_media": ptr(0.04),
	})

	text := app.RenderCompareText(compareOf(t, db, "WEGE3", "ROMI3", "MXRF11"))

	require.Contains(t, text, "Ações")
	require.Contains(t, text, "FIIs")
	require.Less(t, strings.Index(text, "Ações"), strings.Index(text, "FIIs"))

	stocks, fiis := splitAtHeading(t, text, "FIIs")
	require.Contains(t, stocks, "P/L")
	require.NotContains(t, stocks, "Vacância")
	require.Contains(t, fiis, "Vacância")
	require.NotContains(t, fiis, "P/L")
}

func TestRenderCompareText_DeterministicWidth(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")
	report := compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")

	first := app.RenderCompareText(report)
	require.Equal(t, first, app.RenderCompareText(report), "nada consulta o terminal")
	require.NotContains(t, first, "\t", "alinhamento por espaço, não por tabulação")
}

func TestRenderCompareText_FooterDetailMissing(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")
	seedDetail(t, db, "WEGE3", map[domain.MetricID]*float64{"ebit": ptr(5e9)})

	text := app.RenderCompareText(compareOf(t, db, "WEGE3", "ROMI3", "KEPL3"))
	require.Contains(t, text, "goinvest detalhar")
	require.Contains(t, text, "EBIT")
}

func TestRenderCompareText_InvalidTickerFooter(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	text := app.RenderCompareText(compareOf(t, db, "WEGE3", "ZZZZ99999", "ROMI3", "PETR4", "KEPL3"))
	require.Contains(t, text, "ZZZZ99999")
	require.Contains(t, text, "PETR4")
	require.Contains(t, text, "goinvest sync")
}

func TestRenderCompareText_BazinAndAlerts(t *testing.T) {
	db := openTemp(t)
	values := wege3Values()
	values["dy"] = ptr(0.45)
	values["pl"] = ptr(2.5)
	values["cotacao"] = ptr(24.00)
	seed(t, db, "BBAS3", domain.ClassStock, values)
	seedStocks(t, db, "ROMI3", "KEPL3")
	require.NoError(t, db.PutSelic(t.Context(), 0.14, collectedAt, collectedAt))
	seedBazinYears(t, db, "BBAS3", 2021, 2025, 1.20)

	text := app.RenderCompareText(compareOf(t, db, "BBAS3", "ROMI3", "KEPL3"))

	require.NotContains(t, text, "Faixa:",
		"payout acima de 100% derruba o Gordon: sobra o Bazin sozinho, não a faixa")
	require.Contains(t, text, "Preço-teto (Bazin)")
	require.Contains(t, text, "R$ 20,00")
	require.NotContains(t, text, "2025: R$", "a lista anual é exclusiva do show")
	require.NotContains(t, text, "sensibilidade", "a sensibilidade é exclusiva do show")

	require.Contains(t, text, "BBAS3")
	require.Contains(t, text, "ALERTA-01")
	require.Contains(t, text, "112,50%")
	require.Contains(t, text, "pp", "o sufixo do DY−Selic não é cortado")
}

func TestRenderCompareText_NoPercentileInCells(t *testing.T) {
	db := openTemp(t)
	seedPeerGroup(t, db, []string{"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3", "FFFF3"},
		"Bens Industriais", "Máquinas", "Motores", []float64{5, 8, 12, 15, 20, 30})

	text := app.RenderCompareText(compareOf(t, db, "AAAA3", "BBBB3", "CCCC3"))
	require.NotContains(t, text, "n=")
	require.NotRegexp(t, `p\d{1,3} `, text)
}

func TestRenderCompareText_EmptyReport(t *testing.T) {
	require.NotPanics(t, func() { app.RenderCompareText(app.CompareReport{}) })
}

func splitAtHeading(t *testing.T, text, heading string) (before, after string) {
	t.Helper()
	i := strings.Index(text, heading)
	require.GreaterOrEqual(t, i, 0, "cabeçalho %q ausente", heading)
	return text[:i], text[i:]
}

// Truncado, "R$ 180.000.00" vira outro número: 180 milhões lidos como 180 mil.
func TestRenderCompareText_NeverTruncatesNumbers(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	text := app.RenderCompareText(compareOf(t, db, "WEGE3", "ROMI3", "KEPL3"))
	require.Contains(t, text, "R$ 180,0 mi", "liquidez de 180 milhões")
	require.Contains(t, text, "R$ 15,0 bi", "patrimônio de 15 bilhões")
	require.NotContains(t, text, "R$ 180.000.00")
}

func TestRenderCompareText_CompactScaleOnlyForMoney(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	text := app.RenderCompareText(compareOf(t, db, "WEGE3", "ROMI3", "KEPL3"))
	require.Contains(t, text, "30,00", "P/L é razão e sai inteiro")
	require.Contains(t, text, "R$ 52,30", "cotação abaixo de mil não é abreviada")
}

func TestRenderCompareText_ColumnsAlign(t *testing.T) {
	db := openTemp(t)
	seedStocks(t, db, "WEGE3", "ROMI3", "KEPL3")

	lines := strings.Split(app.RenderCompareText(compareOf(t, db, "WEGE3", "ROMI3", "KEPL3")), "\n")
	var widths []int
	for _, l := range lines {
		if strings.HasPrefix(l, "P/L") || strings.HasPrefix(l, "Patrimônio") || strings.HasPrefix(l, "Cotação") {
			widths = append(widths, len([]rune(strings.TrimRight(l, " "))))
		}
	}
	require.Len(t, widths, 3)

	starts := columnStarts(t, lines)
	require.Len(t, starts, 3, "três colunas de ticker")
}

func columnStarts(t *testing.T, lines []string) []int {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(l, "WEGE3") || !strings.Contains(l, "KEPL3") {
			continue
		}
		var out []int
		for _, ticker := range []string{"WEGE3", "ROMI3", "KEPL3"} {
			out = append(out, strings.Index(l, ticker))
		}
		return out
	}
	t.Fatal("linha de cabeçalho de tickers ausente")
	return nil
}
