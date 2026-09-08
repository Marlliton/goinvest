package app

import (
	"fmt"
	"strings"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
)

// A largura sai do conteúdo da própria tabela, nunca do terminal: medir a
// janela tornaria a saída dependente de onde ela roda, e o golden deixaria de
// valer como prova. Truncar para uma largura fixa era pior que largo demais:
// "R$ 180.000.00" não é um R$ 180.000.000,00 cortado, é outro número.
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
	// O grupo de pares fica no cabeçalho e não na célula: repeti-lo em cada
	// uma multiplicaria a largura por linha sem acrescentar leitura.
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
	if reason := col.NotApplicable[m.ID]; reason != "" {
		return markNotApplicable
	}
	v, ok := col.Values[m.ID]
	if !ok || v == nil {
		return markAbsent
	}
	cell := formatCompact(*v, m.Unit)
	if m.ID == dividendYieldID && col.SelicDelta != nil {
		cell += " " + formatSignedBR(*col.SelicDelta*100, 1) + "pp"
	}
	return cell
}

// Preço-teto e alertas são por ativo, não por métrica: eles não cabem numa
// célula e sairiam mentindo se coubessem.
func compareHighlights(col CompareColumn) string {
	var b strings.Builder

	if col.Bazin != nil && col.Bazin.NotApplicableReason == "" {
		fmt.Fprintf(&b, "\n%s · %s: %s · ágio/deságio %s%%\n",
			col.Ticker, bazinLabel,
			formatValue(col.Bazin.Ceiling, domain.UnitBRL),
			formatSignedBR(col.Bazin.PremiumDiscount*100, 2))
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

// Oito colunas com R$ 15.000.000.000,00 não cabem em tela nenhuma, e cortar o
// número o transforma em outro. A escala abreviada preserva a ordem de
// grandeza, que é o que a comparação lado a lado precisa. O valor cheio
// continua em 'goinvest show', onde o propósito é auditoria.
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

// A contagem é de runas: acento e "—" ocupam mais de um byte, e medir bytes
// desalinharia toda coluna à direita de um rótulo acentuado.
func width(s string) int { return len([]rune(s)) + compareGutter }

func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(0, w-len([]rune(s))))
}
