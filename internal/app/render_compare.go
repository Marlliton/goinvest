package app

import (
	"fmt"
	"strings"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
)

// A largura sai do conteúdo, nunca do terminal: medir a janela faria o golden
// depender de onde o teste roda.
const (
	compareGutter    = 2
	markNotEvaluated = "não avaliado"
)

func RenderCompareText(r CompareReport) string {
	var b strings.Builder

	if r.Header.SelicRate != nil {
		fmt.Fprintf(&b, "Selic: %s (%s)\n\n",
			formatValue(*r.Header.SelicRate, domain.UnitPercent),
			r.Header.SelicAt.Format("02/01/2006"))
	}

	for i, table := range r.Tables {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(compareTable(table, r.DetailMissingFor))
	}

	for _, table := range r.Tables {
		for _, col := range table.Columns {
			b.WriteString(compareHighlights(col))
		}
	}

	b.WriteString(compareFooter(r))
	return b.String()
}

func compareTable(table CompareTable, missing []domain.MetricID) string {
	cells := make([][]string, len(table.Metrics))
	labelWidth := 0
	widths := make([]int, len(table.Columns))
	for i, col := range table.Columns {
		widths[i] = max(width(col.Ticker), width(col.PeerGroupLabel))
	}
	for r, m := range table.Metrics {
		labelWidth = max(labelWidth, width(m.Label))
		cells[r] = make([]string, len(table.Columns))
		for c, col := range table.Columns {
			cells[r][c] = compareCell(col, m, missing)
			widths[c] = max(widths[c], width(cells[r][c]))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", sectionLabel(table.Class))

	row := func(label string, values []string) {
		b.WriteString(pad(label, labelWidth))
		for i, v := range values {
			b.WriteString(pad(v, widths[i]))
		}
		b.WriteString("\n")
	}

	row("", tickers(table.Columns))
	if anyPeerGroup(table.Columns) {
		row("", peerGroups(table.Columns))
	}
	for r, m := range table.Metrics {
		row(m.Label, cells[r])
	}
	return b.String()
}

func tickers(columns []CompareColumn) []string {
	out := make([]string, 0, len(columns))
	for _, c := range columns {
		out = append(out, c.Ticker)
	}
	return out
}

func peerGroups(columns []CompareColumn) []string {
	out := make([]string, 0, len(columns))
	for _, c := range columns {
		out = append(out, c.PeerGroupLabel)
	}
	return out
}

func compareCell(col CompareColumn, m catalog.Metric, missing []domain.MetricID) string {
	for _, id := range missing {
		if id == m.ID {
			return markNotEvaluated
		}
	}
	cell, ok := col.Cells[m.ID]
	if ok && cell.NotApplicableReason != "" {
		return markNotApplicable
	}
	if !ok || cell.Value == nil {
		return markAbsent
	}
	out := formatCompact(*cell.Value, m.Unit)
	if m.ID == dividendYieldID && col.SelicDelta != nil {
		out += " " + formatSignedBR(*col.SelicDelta*100, 1) + "pp"
	}
	return out
}

func compareHighlights(col CompareColumn) string {
	var b strings.Builder

	if col.Bazin != nil && col.Bazin.NotApplicableReason == "" {
		if col.Bazin.Gordon != nil && col.Bazin.Gordon.NotApplicableReason == "" {
			lo, hi := min(col.Bazin.Ceiling, col.Bazin.Gordon.Ceiling), max(col.Bazin.Ceiling, col.Bazin.Gordon.Ceiling)
			fmt.Fprintf(&b, "\n%s · Faixa: %s (Bazin) a %s (Gordon) · cotação %s · ágio/deságio %s%%\n",
				col.Ticker,
				formatValue(lo, domain.UnitBRL), formatValue(hi, domain.UnitBRL),
				formatValue(col.Bazin.CurrentPrice, domain.UnitBRL),
				formatSignedBR(col.Bazin.PremiumDiscount*100, 2))
		} else {
			fmt.Fprintf(&b, "\n%s · %s: %s · ágio/deságio %s%%\n",
				col.Ticker, bazinLabel,
				formatValue(col.Bazin.Ceiling, domain.UnitBRL),
				formatSignedBR(col.Bazin.PremiumDiscount*100, 2))
		}
	}

	fired := false
	for _, f := range col.Alerts {
		if f.Status != evaluate.StatusFired {
			continue
		}
		if !fired {
			fmt.Fprintf(&b, "\n%s · alertas\n", col.Ticker)
			fired = true
		}
		fmt.Fprintf(&b, "  %s %s: %s\n", markAlert, f.ID, f.Rule)
		fmt.Fprintf(&b, "    %s\n", alertNumbers(f.Numbers))
	}
	return b.String()
}

func compareFooter(r CompareReport) string {
	var b strings.Builder

	if len(r.DetailMissingFor) > 0 {
		fmt.Fprintf(&b, "\n%s não avaliado por falta de coleta profunda: %s\n",
			markAbsent, strings.Join(metricLabels(r), ", "))
		b.WriteString("  rode 'goinvest detalhar TICKER' para completar\n")
	}

	if len(r.Invalid) > 0 {
		b.WriteString("\nFora da comparação\n")
		for _, s := range r.Invalid {
			fmt.Fprintf(&b, "  %s: %s\n", s.Ticker, s.Reason)
		}
	}
	return b.String()
}

func metricLabels(r CompareReport) []string {
	labels := make([]string, 0, len(r.DetailMissingFor))
	for _, id := range r.DetailMissingFor {
		labels = append(labels, metricLabel(r, id))
	}
	return labels
}

func metricLabel(r CompareReport, id domain.MetricID) string {
	for _, table := range r.Tables {
		for _, m := range table.Metrics {
			if m.ID == id {
				return m.Label
			}
		}
	}
	return string(id)
}

func anyPeerGroup(columns []CompareColumn) bool {
	for _, c := range columns {
		if c.PeerGroupLabel != "" {
			return true
		}
	}
	return false
}

// Abreviar em vez de truncar: "R$ 180.000.00" não é um R$ 180.000.000,00
// cortado, é outro número. O valor cheio continua em 'goinvest show'.
func formatCompact(v float64, unit domain.Unit) string {
	if unit != domain.UnitBRL {
		return formatValue(v, unit)
	}
	for _, s := range []struct {
		threshold float64
		suffix    string
	}{
		{1e9, " bi"},
		{1e6, " mi"},
		{1e3, " mil"},
	} {
		if abs(v) >= s.threshold {
			return "R$ " + formatBR(v/s.threshold, 1) + s.suffix
		}
	}
	return formatValue(v, unit)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// Contagem em runas: medir bytes desalinharia a coluna após rótulo acentuado.
func width(s string) int { return len([]rune(s)) + compareGutter }

func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-len([]rune(s))))
}
