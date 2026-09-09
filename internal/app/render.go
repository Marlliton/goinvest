package app

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
)

const (
	markAbsent        = "—"
	markNotApplicable = "◌"
	markDerived       = "ƒ"
	markFallback      = "*"
	markAlert         = "⚠"
)

func RenderText(r Report) string {
	var b strings.Builder

	fmt.Fprintf(&b, "%s · %s\n", r.Ticker, classLabel(r.Class))
	b.WriteString(headerLine(r.Header) + "\n")
	if r.Header.Stale {
		fmt.Fprintf(&b, "⚠ dado de %s · rode 'goinvest sync'\n", r.Header.FetchedAt.Format("02/01"))
	}
	if r.Header.Inactive {
		fmt.Fprintf(&b, "⚠ papel %s · fora de rankings e comparações\n", liquidityText(r.Header))
	}
	b.WriteString(sectorLine(r.Header) + "\n")
	b.WriteString(opportunityCostText(r.OpportunityCost))
	if r.Header.IncompleteRegistry > 0 {
		fmt.Fprintf(&b, "cadastro incompleto: %d de %d\n",
			r.Header.IncompleteRegistry, r.Header.TotalInClass)
	}

	if r.Header.PeerGroupLabel != "" {
		fmt.Fprintf(&b, "Comparado com %s (%d papéis líquidos)\n",
			r.Header.PeerGroupLabel, r.Header.PeerGroupN)
	}

	sawAbsent, sawDerived, sawFallback, sawSmallerPeerN := false, false, false, false
	sawNotApplicable := false
	for _, block := range r.Blocks {
		fmt.Fprintf(&b, "\n%s\n", block.Label)
		for _, line := range block.Lines {
			b.WriteString("  " + line.Label + ": ")
			if line.NotApplicableReason != "" {
				sawNotApplicable = true
				fmt.Fprintf(&b, "%s (%s)\n", markNotApplicable, line.NotApplicableReason)
				continue
			}
			if line.Value == nil {
				sawAbsent = true
				b.WriteString(markAbsent + "\n")
				continue
			}
			b.WriteString(formatValue(*line.Value, line.Unit))
			if line.SelicDelta != nil {
				fmt.Fprintf(&b, " · DY−Selic: %spp", formatSignedBR(*line.SelicDelta*100, 2))
			}
			if line.Percentile != nil {
				fmt.Fprintf(&b, " · p%d · n=%d", int(*line.Percentile*100), *line.PeerN)
				if *line.PeerN < r.Header.PeerGroupN {
					sawSmallerPeerN = true
				}
				if line.FellBackToMarket {
					sawFallback = true
					b.WriteString(" " + markFallback)
				}
			}
			if line.Derived {
				sawDerived = true
				fmt.Fprintf(&b, " %s (%s)", markDerived, line.Formula)
			}
			if len(line.AlertMarks) > 0 {
				b.WriteString(" " + markAlert)
			}
			if line.ReferenceAt != nil && !sameDay(*line.ReferenceAt, r.Header.ReferenceAt) {
				fmt.Fprintf(&b, " · ref %s", line.ReferenceAt.Format("02/01"))
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(expectedReturnText(r.ExpectedReturn))
	if r.Bazin != nil {
		b.WriteString("\n" + bazinText(*r.Bazin))
	}
	if alerts := alertsText(r.Alerts); alerts != "" {
		b.WriteString("\n" + alerts)
	}

	if legend := legend(sawAbsent, sawNotApplicable, sawDerived, sawFallback, sawSmallerPeerN); legend != "" {
		fmt.Fprintf(&b, "\n%s\n", legend)
	}
	return b.String()
}

func alertsText(findings []evaluate.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		if f.Status != evaluate.StatusFired {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("Alertas\n")
		}
		fmt.Fprintf(&b, "  %s %s: %s\n", markAlert, f.ID, f.Rule)
		fmt.Fprintf(&b, "    %s\n", alertNumbers(f.Numbers))
	}
	return b.String()
}

func alertNumbers(numbers map[string]float64) string {
	parts := make([]string, 0, len(numbers))
	for _, name := range slices.Sorted(maps.Keys(numbers)) {
		f := alertNumberFormats[name]
		if f.label == "" {
			f.label = name
		}
		parts = append(parts, fmt.Sprintf("%s: %s", f.label, formatValue(numbers[name], f.unit)))
	}
	return strings.Join(parts, " · ")
}

// Sem a unidade explícita, um P/VP de 0,60 sairia como 60%.
var alertNumberFormats = map[string]struct {
	label string
	unit  domain.Unit
}{
	"payout":               {"payout", domain.UnitPercent},
	"pvp":                  {"P/VP", domain.UnitRatio},
	"roe":                  {"ROE", domain.UnitPercent},
	"ke":                   {"retorno exigido", domain.UnitPercent},
	"lucro_liquido":        {"Lucro Líquido", domain.UnitBRL},
	"ebit":                 {"EBIT", domain.UnitBRL},
	"venda_sobre_receita":  {"venda de ativos ÷ receita", domain.UnitPercent},
	"rendimento_sobre_ffo": {"rendimento ÷ FFO", domain.UnitPercent},
	"vacancia_media":       {"vacância", domain.UnitPercent},
	"limiar":               {"limiar do segmento", domain.UnitPercent},
	"dy_percentil":         {"percentil do DY no segmento", domain.UnitPercent},
}

func opportunityCostText(v *OpportunityCostView) string {
	if v == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("Custo de oportunidade\n")
	fmt.Fprintf(&b, "  Premissa: %s\n", taxPremiseLine(v.TaxAssumption))
	if v.IPCANominalizationNote != "" {
		fmt.Fprintf(&b, "  %s\n", v.IPCANominalizationNote)
	}
	for _, a := range v.Anchors {
		if a.NotEvaluatedReason != "" {
			fmt.Fprintf(&b, "  %s: %s (%s)\n", a.Label, markAbsent, a.NotEvaluatedReason)
			continue
		}
		fmt.Fprintf(&b, "  %s: %s bruto", a.Label, formatValue(*a.RateGross, domain.UnitPercent))
		if a.ReferenceAt != nil {
			fmt.Fprintf(&b, " (%s)", a.ReferenceAt.Format("02/01/2006"))
		}
		fmt.Fprintf(&b, " · líquido %s", formatValue(*a.RateNet, domain.UnitPercent))
		if a.SpreadNet != nil {
			fmt.Fprintf(&b, " · spread %spp", formatSignedBR(*a.SpreadNet*100, 2))
		}
		b.WriteString("\n")
	}
	return b.String()
}

func taxPremiseLine(t TaxAssumptionView) string {
	dividendLabel := "tributado a " + formatValue(t.AliquotaAtivo, domain.UnitPercent)
	if t.AliquotaAtivo == 0 {
		dividendLabel = "isento"
	}
	source := "config em " + t.ConfigPath
	if !t.FromFile {
		source = "arquivo ausente; usando padrão embutido, crie " + t.ConfigPath + " para mudar"
	}
	return fmt.Sprintf("considerando dividendo %s e renda fixa tributada a %s (%s)",
		dividendLabel, formatValue(t.AliquotaRendaFixa, domain.UnitPercent), source)
}

const expectedReturnLabel = "Retorno esperado"

func expectedReturnText(v *ExpectedReturnView) string {
	if v == nil {
		return ""
	}
	if v.NotApplicableReason != "" {
		return fmt.Sprintf("\n%s: não aplicável (%s)\n", expectedReturnLabel, v.NotApplicableReason)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "\n%s\n", expectedReturnLabel)
	fmt.Fprintf(&b, "  Lucro/preço: %s · Distribuído (DY): %s · Retido: %s\n",
		formatValue(v.EarningsYield, domain.UnitPercent),
		formatValue(v.Distributed, domain.UnitPercent),
		formatValue(v.Retained, domain.UnitPercent))
	fmt.Fprintf(&b, "  Crescimento implícito (g = ROE × retenção): %s\n",
		formatValue(v.ImpliedGrowth, domain.UnitPercent))
	fmt.Fprintf(&b, "  Retorno esperado total: %s · Payback: %s anos (P/L lido como tempo)\n",
		formatValue(v.TotalReturn, domain.UnitPercent),
		formatBR(v.PaybackYears, 1))
	return b.String()
}

const bazinLabel = "Preço-teto (Bazin)"

func bazinText(v BazinView) string {
	// Motivo entre parênteses: "—" já é o símbolo de "fonte não informa" na legenda.
	if v.NotApplicableReason != "" {
		return fmt.Sprintf("%s: não aplicável (%s)\n", bazinLabel, v.NotApplicableReason)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", bazinLabel)
	fmt.Fprintf(&b, "  Teto: %s · Cotação: %s · Ágio/deságio: %s%%\n",
		formatValue(v.Ceiling, domain.UnitBRL),
		formatValue(v.CurrentPrice, domain.UnitBRL),
		formatSignedBR(v.PremiumDiscount*100, 2))

	fmt.Fprintf(&b, "  Anos usados: %d de %d", v.YearsUsed, bazin.WindowYears)
	for _, year := range v.AtypicalYears {
		fmt.Fprintf(&b, " · ano %d concentrado", year)
	}
	b.WriteString("\n")

	if v.Gordon != nil {
		if v.Gordon.NotApplicableReason != "" {
			fmt.Fprintf(&b, "  Faixa: Bazin %s — Gordon não aplicável (%s)\n",
				formatValue(v.Ceiling, domain.UnitBRL), v.Gordon.NotApplicableReason)
		} else {
			lo, hi := min(v.Ceiling, v.Gordon.Ceiling), max(v.Ceiling, v.Gordon.Ceiling)
			fmt.Fprintf(&b, "  Faixa: %s (Bazin) a %s (Gordon) — cotação de hoje %s, %s\n",
				formatValue(lo, domain.UnitBRL), formatValue(hi, domain.UnitBRL),
				formatValue(v.CurrentPrice, domain.UnitBRL), rangePosition(v.CurrentPrice, lo, hi))
			fmt.Fprintf(&b, "  Gordon: Ke %s · g %s\n",
				formatValue(v.Gordon.RequiredReturn, domain.UnitPercent),
				formatValue(v.Gordon.ImpliedGrowth, domain.UnitPercent))
			for _, s := range v.Gordon.Sensitivity {
				fmt.Fprintf(&b, "  sensibilidade (%s, g=%s): %s\n",
					s.Label, formatValue(s.Growth, domain.UnitPercent), formatValue(s.Ceiling, domain.UnitBRL))
			}
		}
	}
	if v.TwelveMonthYield != nil || v.MedianDividendYield != nil {
		fmt.Fprintf(&b, "  DY 12m: %s · DY mediano 5a: %s\n",
			optPercent(v.TwelveMonthYield), optPercent(v.MedianDividendYield))
	}

	for _, y := range v.Years {
		fmt.Fprintf(&b, "  %d: %s\n", y.Year, formatValue(y.Total, domain.UnitBRL))
	}
	return b.String()
}

func rangePosition(price, lo, hi float64) string {
	switch {
	case price < lo:
		return "abaixo da faixa"
	case price > hi:
		return "acima da faixa"
	default:
		return "dentro da faixa"
	}
}

func optPercent(v *float64) string {
	if v == nil {
		return markAbsent
	}
	return formatValue(*v, domain.UnitPercent)
}

func legend(sawAbsent, sawNotApplicable, sawDerived, sawFallback, sawSmallerPeerN bool) string {
	var parts []string
	if sawAbsent {
		parts = append(parts, markAbsent+" = fonte não informa")
	}
	if sawNotApplicable {
		parts = append(parts, markNotApplicable+" = não se aplica a este ativo/setor")
	}
	if sawDerived {
		parts = append(parts, markDerived+" = calculado por goinvest")
	}
	if sawFallback {
		parts = append(parts, markFallback+" = comparado com o mercado inteiro; o setor tem poucos papéis com esta métrica")
	}
	if sawSmallerPeerN {
		parts = append(parts, "n = quantos papéis do grupo têm este indicador; pode ser menor que o total do cabeçalho")
	}
	return strings.Join(parts, " · ")
}

func sameDay(a time.Time, b *time.Time) bool {
	if b == nil {
		return a.IsZero()
	}
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

const sectorLevelSep = " / "

func sectorLine(h HeaderView) string {
	if h.Sector == "" {
		return "Setor: desconhecido"
	}
	levels := []string{h.Sector}
	if h.Subsector != "" {
		levels = append(levels, h.Subsector)
	}
	if h.Segment != "" {
		levels = append(levels, h.Segment)
	}
	return "Setor: " + strings.Join(levels, sectorLevelSep)
}

func liquidityText(h HeaderView) string {
	if h.LastLiquidAt == nil {
		return "sem liquidez registrada"
	}
	return "sem liquidez desde " + h.LastLiquidAt.Format("02/01/2006")
}

func headerLine(h HeaderView) string {
	if h.ReferenceAt != nil {
		return fmt.Sprintf("Balanço %s · coletado %s", h.ReferenceAt.Format("02/01/2006"), ageText(h))
	}
	return fmt.Sprintf("Referência: desconhecida (fonte não informa) · coletado %s", ageText(h))
}

func ageText(h HeaderView) string {
	switch days := int(h.Age.Hours() / 24); days {
	case 0:
		return "hoje"
	case 1:
		return "há 1 dia"
	default:
		return fmt.Sprintf("há %d dias", days)
	}
}

func classLabel(c domain.AssetClass) string {
	if c == domain.ClassFII {
		return "FII"
	}
	return "Ação"
}

func formatValue(v float64, unit domain.Unit) string {
	switch unit {
	case domain.UnitPercent:
		return formatBR(v*100, 2) + "%"
	case domain.UnitBRL:
		return "R$ " + formatBR(v, 2)
	case domain.UnitCount:
		return formatBR(v, 0)
	default:
		return formatBR(v, 2)
	}
}

func formatSignedBR(v float64, decimals int) string {
	if v >= 0 {
		return "+" + formatBR(v, decimals)
	}
	return formatBR(v, decimals)
}

func formatBR(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	sign := ""
	if strings.HasPrefix(s, "-") {
		sign, s = "-", s[1:]
	}

	whole, frac, hasFrac := strings.Cut(s, ".")
	var b strings.Builder
	for i, d := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(d)
	}

	out := sign + b.String()
	if hasFrac {
		out += "," + frac
	}
	return out
}

const (
	fallbackToMarket = "a referência de mercado"
	fallbackToSector = "a referência do setor"
)

func RenderSectors(groups []ClassSectors) string {
	var b strings.Builder
	for i, g := range groups {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "%s\n", sectionLabel(g.Class))
		if g.IncompleteRegistry > 0 {
			fmt.Fprintf(&b, "cadastro incompleto: %d de %d\n", g.IncompleteRegistry, g.TotalAssets)
		}
		for _, s := range g.Groups {
			b.WriteString("  " + sectorGroupLine(s, fallbackToMarket) + "\n")
		}
	}
	return b.String()
}

func RenderSectorsDescend(sector string, d SectorDescend) string {
	if d.SingleLevel {
		return fmt.Sprintf("%s — %s: taxonomia de FII tem um nível só, não há subsetor para descer\n",
			sector, plural(d.N, "papel líquido", "papéis líquidos"))
	}

	fallback := fallbackToSector
	if d.BelowThreshold {
		fallback = fallbackToMarket
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s · subsetores\n", sector)
	if d.AlsoFII {
		b.WriteString("este nome também é setor de FII, taxonomia de nível único\n")
	}
	for _, s := range d.Groups {
		b.WriteString("  " + sectorGroupLine(s, fallback) + "\n")
	}
	return b.String()
}

func sectionLabel(c domain.AssetClass) string {
	if c == domain.ClassFII {
		return "FIIs"
	}
	return "Ações"
}

func sectorGroupLine(s SectorGroup, fallback string) string {
	if s.BelowThreshold {
		return fmt.Sprintf("%s — %s: percentil cai para %s",
			s.Name, plural(s.N, "papel líquido", "papéis líquidos"), fallback)
	}
	return fmt.Sprintf("%s — %s", s.Name, plural(s.N, "ativo líquido", "ativos líquidos"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
