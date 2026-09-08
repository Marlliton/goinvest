package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func line(y int, m time.Month, d int, kind domain.DividendType, perShare float64) app.DividendLine {
	return app.DividendLine{
		ExDate:        time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Type:          kind,
		ValuePerShare: perShare,
	}
}

func TestRenderDividends_AnnualSum(t *testing.T) {
	view := app.DividendsView{
		Ticker: "BBAS3",
		Lines: []app.DividendLine{
			line(2024, time.September, 1, domain.DividendJCP, 0.10),
			line(2024, time.June, 1, domain.DividendCash, 0.20),
			line(2024, time.March, 1, domain.DividendJCP, 0.30),
			line(2023, time.December, 1, domain.DividendCash, 0.50),
			line(2023, time.June, 1, domain.DividendJCP, 0.25),
		},
	}

	text := app.RenderDividends(view)
	require.Contains(t, text, "2024: R$ 0,60")
	require.Contains(t, text, "2023: R$ 0,75")
	require.Less(t, strings.Index(text, "2024: "), strings.Index(text, "2023: "),
		"o ano mais recente vem primeiro, como as linhas")
}

func TestRenderDividends_SeparatesTypes(t *testing.T) {
	view := app.DividendsView{
		Ticker: "BBAS3",
		Lines: []app.DividendLine{
			line(2024, time.September, 1, domain.DividendJCP, 0.10),
			line(2024, time.June, 1, domain.DividendCash, 0.20),
		},
	}

	text := app.RenderDividends(view)
	require.Contains(t, text, "01/09/2024 · JCP · R$ 0,10")
	require.Contains(t, text, "01/06/2024 · DIVIDENDO · R$ 0,20")
}

// Um provento por ação de lote de mil ações arredondaria para zero com duas
// casas fixas, e "R$ 0,00" se leria como "não pagou nada".
func TestRenderDividends_SmallValueNeverRoundsToZero(t *testing.T) {
	view := app.DividendsView{
		Ticker: "BBAS3",
		Lines:  []app.DividendLine{line(2024, time.March, 14, domain.DividendJCP, 0.0004398)},
	}

	text := app.RenderDividends(view)
	require.Contains(t, text, "R$ 0,0004398")
	require.NotContains(t, text, "R$ 0,00\n")
}

func TestRenderDividends_KeepsTwoDecimalsForOrdinaryValues(t *testing.T) {
	view := app.DividendsView{
		Ticker: "ITSA4",
		Lines: []app.DividendLine{
			line(2024, time.March, 14, domain.DividendCash, 10.5),
			line(2024, time.February, 14, domain.DividendCash, 1),
		},
	}

	text := app.RenderDividends(view)
	require.Contains(t, text, "R$ 10,50")
	require.Contains(t, text, "R$ 1,00")
}
