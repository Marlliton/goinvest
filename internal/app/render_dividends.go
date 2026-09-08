package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Com duas casas fixas, um provento de R$ 0,00007 por ação viraria "R$ 0,00".
const maxDividendDecimals = 8

func RenderDividends(v DividendsView) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s · proventos\n\n", v.Ticker)
	for _, line := range v.Lines {
		fmt.Fprintf(&b, "%s · %s · R$ %s\n",
			line.ExDate.Format("02/01/2006"), line.Type, formatDividend(line.ValuePerShare))
	}

	byYear := make(map[int]float64, len(v.Lines))
	for _, line := range v.Lines {
		byYear[line.ExDate.Year()] += line.ValuePerShare
	}

	b.WriteString("\nSoma por ano\n")
	for _, year := range slices.Backward(slices.Sorted(maps.Keys(byYear))) {
		fmt.Fprintf(&b, "  %d: R$ %s\n", year, formatDividend(byYear[year]))
	}
	return b.String()
}

func formatDividend(v float64) string {
	s := strings.TrimRight(formatBR(v, maxDividendDecimals), "0")
	whole, frac, _ := strings.Cut(s, ",")
	if len(frac) < 2 {
		frac += strings.Repeat("0", 2-len(frac))
	}
	return whole + "," + frac
}
