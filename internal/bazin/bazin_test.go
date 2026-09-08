package bazin_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

func event(exDate string, raw float64, kind domain.DividendType) domain.DividendEvent {
	d, err := time.Parse("02/01/2006", exDate)
	if err != nil {
		panic(err)
	}
	return domain.DividendEvent{
		Ticker:           "BBAS3",
		ExDate:           d,
		Type:             kind,
		ValuePerShareRaw: raw,
		SharesFactor:     1,
		Source:           "fundamentus:proventos",
		FetchedAt:        now,
	}
}

const (
	jcp  = domain.DividendJCP
	cash = domain.DividendCash
)

// Eventos reais de BBAS3 capturados na fixture proventos_bbas3.html. Os totais
// esperados são os que a própria fonte publica em #resultado-anual, o que faz
// deste um teste dourado contra a fonte, não contra uma soma feita à mão.
func bbas3Events() []domain.DividendEvent {
	return []domain.DividendEvent{
		event("13/12/2021", 0.1750, jcp), event("22/11/2021", 0.3937, jcp),
		event("13/09/2021", 0.1847, jcp), event("23/08/2021", 0.3456, jcp),
		event("11/06/2021", 0.1685, jcp), event("21/05/2021", 0.3401, jcp),
		event("21/05/2021", 0.0743, cash), event("11/03/2021", 0.1457, jcp),
		event("22/02/2021", 0.4345, jcp),

		event("12/12/2022", 0.3455, jcp), event("21/11/2022", 0.6345, jcp),
		event("21/11/2022", 0.1702, cash), event("12/09/2022", 0.2737, jcp),
		event("22/08/2022", 0.5707, jcp), event("22/08/2022", 0.2002, cash),
		event("13/06/2022", 0.2503, jcp), event("23/05/2022", 0.5177, jcp),
		event("23/05/2022", 0.1553, cash), event("14/03/2022", 0.2106, jcp),
		event("02/03/2022", 0.4542, jcp), event("02/03/2022", 0.3558, cash),

		event("11/12/2023", 0.3423, jcp), event("21/11/2023", 0.6862, jcp),
		event("21/11/2023", 0.1020, cash), event("11/09/2023", 0.3342, jcp),
		event("21/08/2023", 0.6547, jcp), event("21/08/2023", 0.1437, cash),
		event("12/06/2023", 0.3386, jcp), event("01/06/2023", 0.6544, jcp),
		event("01/06/2023", 0.1230, cash), event("13/03/2023", 0.3520, jcp),
		event("23/02/2023", 0.5735, jcp), event("23/02/2023", 0.2355, cash),

		event("11/12/2024", 0.1765, jcp), event("25/11/2024", 0.4833, jcp),
		event("11/09/2024", 0.1866, jcp), event("21/08/2024", 0.3145, jcp),
		event("21/08/2024", 0.1519, cash), event("13/06/2024", 0.2042, jcp),
		event("11/06/2024", 0.2932, jcp), event("11/06/2024", 0.1648, cash),
		event("11/03/2024", 0.4100, jcp), event("21/02/2024", 0.6136, jcp),
		event("21/02/2024", 0.2208, cash),

		event("02/12/2025", 0.0458, jcp), event("01/12/2025", 0.0719, jcp),
		event("02/06/2025", 0.3343, jcp), event("02/06/2025", 0.0904, jcp),
		event("11/03/2025", 0.3426, jcp), event("11/03/2025", 0.1494, jcp),
		event("11/03/2025", 0.1360, cash),
	}
}

// Totais publicados em #resultado-anual da mesma fixture.
var bbas3AnnualGross = map[int]float64{
	2021: 2.262, 2022: 4.139, 2023: 4.540, 2024: 3.219, 2025: 1.170,
}

func grossEvents() []domain.DividendEvent {
	events := bbas3Events()
	for i := range events {
		events[i].Type = cash
	}
	return events
}

func TestCompute_MatchesAnnualTable(t *testing.T) {
	got, ok := bazin.Compute(grossEvents(), now)
	require.True(t, ok)
	require.Len(t, got.Years, 5)

	for _, y := range got.Years {
		require.InDelta(t, bbas3AnnualGross[y.Year], y.Total, 5e-4, "ano %d", y.Year)
	}
	require.Equal(t, []int{2025, 2024, 2023, 2022, 2021}, years(got))
	require.Empty(t, got.AtypicalYears)
}

func TestCompute_JCPNetOfTax(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 1.00, jcp),
		event("10/03/2024", 1.00, jcp),
		event("10/03/2025", 1.00, jcp),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	for _, y := range got.Years {
		require.InDelta(t, 0.85, y.Total, 1e-9, "ano %d", y.Year)
	}
	require.InDelta(t, 0.85, got.AverageAnnual, 1e-9)
}

func TestCompute_DividesBySharesFactor(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 1.00, cash),
		event("10/03/2024", 1.00, cash),
		event("10/03/2025", 1.00, cash),
	}
	events[0].ValuePerShareRaw = 1000
	events[0].SharesFactor = 1000

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.InDelta(t, 1.0, got.AverageAnnual, 1e-9)
}

func TestCompute_FloorOfThreeYears(t *testing.T) {
	two := []domain.DividendEvent{
		event("10/03/2024", 1.00, cash),
		event("10/03/2025", 1.00, cash),
	}
	_, ok := bazin.Compute(two, now)
	require.False(t, ok)

	three := append(two, event("10/03/2023", 1.00, cash))
	_, ok = bazin.Compute(three, now)
	require.True(t, ok)
}

// O ano corrente nunca entra na janela: um exercício ainda em curso soma menos
// que os fechados e puxaria a média para baixo sem nada explicar por quê.
func TestCompute_IgnoresCurrentYear(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 1.00, cash),
		event("10/03/2024", 1.00, cash),
		event("10/03/2025", 1.00, cash),
		event("10/03/2026", 0.10, cash),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.Equal(t, []int{2025, 2024, 2023}, years(got))
	require.InDelta(t, 1.0, got.AverageAnnual, 1e-9)
}

func TestCompute_LessThanFiveYears(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2022", 1.00, cash),
		event("10/03/2023", 1.00, cash),
		event("10/03/2024", 1.00, cash),
		event("10/03/2025", 1.00, cash),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.Equal(t, 4, got.YearsAvailable)
	require.Len(t, got.Years, 4)
}

func TestCompute_UsesOnlyFiveMostRecentClosedYears(t *testing.T) {
	var events []domain.DividendEvent
	for year := 2018; year <= 2025; year++ {
		events = append(events, event("10/03/"+strconv.Itoa(year), 1.00, cash))
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.Equal(t, 5, got.YearsAvailable)
	require.Equal(t, []int{2025, 2024, 2023, 2022, 2021}, years(got))
}

func TestCompute_AtypicalYear(t *testing.T) {
	events := []domain.DividendEvent{
		// 2021 destoa dos demais no total, mas nenhum evento isolado passa de
		// metade do próprio ano: não é concentração.
		event("10/03/2021", 1.60, cash), event("10/06/2021", 1.60, cash),
		event("10/09/2021", 1.60, cash),

		event("10/03/2022", 0.50, cash), event("10/06/2022", 0.50, cash),
		event("10/03/2023", 0.50, cash), event("10/06/2023", 0.50, cash),
		event("10/03/2024", 0.50, cash), event("10/06/2024", 0.50, cash),

		// 2025 tem o mesmo total dos anos vizinhos, mas um único evento
		// responde por 90% dele.
		event("10/03/2025", 0.90, cash), event("10/06/2025", 0.10, cash),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.Equal(t, []int{2025}, got.AtypicalYears)
}

func TestCompute_CeilingFormula(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 1.00, cash),
		event("10/03/2024", 2.00, cash),
		event("10/03/2025", 3.00, cash),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.InDelta(t, 2.0, got.AverageAnnual, 1e-9)
	require.Equal(t, got.AverageAnnual/0.06, got.Ceiling)
}

// Fator zero derruba a linha em vez de virar infinito: é a mesma regra que a
// leitura da série de proventos já aplica.
func TestCompute_DropsNonPositiveSharesFactor(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 0.50, cash), event("10/09/2023", 0.50, cash),
		event("10/03/2024", 0.50, cash), event("10/09/2024", 0.50, cash),
		event("10/03/2025", 0.50, cash), event("10/09/2025", 0.50, cash),
		event("11/03/2025", 5.00, cash),
	}
	events[6].SharesFactor = 0

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.InDelta(t, 1.0, got.AverageAnnual, 1e-9)
	require.Empty(t, got.AtypicalYears)
}

// A regra é "um evento responde por mais da metade do que o ano pagou", e um
// pagador anual satisfaz isso sempre. É o preço de aplicar o limiar na unidade
// em que ele foi definido; o alternativo seria um limiar novo sem fonte.
func TestCompute_SingleEventYearIsAtypical(t *testing.T) {
	events := []domain.DividendEvent{
		event("10/03/2023", 1.00, cash),
		event("10/03/2024", 1.00, cash),
		event("10/03/2025", 1.00, cash),
	}

	got, ok := bazin.Compute(events, now)
	require.True(t, ok)
	require.Equal(t, []int{2025, 2024, 2023}, got.AtypicalYears)
}

func TestCompute_NoEvents(t *testing.T) {
	_, ok := bazin.Compute(nil, now)
	require.False(t, ok)
}

func years(r bazin.Result) []int {
	out := make([]int, 0, len(r.Years))
	for _, y := range r.Years {
		out = append(out, y.Year)
	}
	return out
}
