package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/marlliton/goinvest/internal/domain"
)

const (
	markAbsent        = "—"
	markNotApplicable = "◌"
	markDerived       = "ƒ"
	markFallback      = "*"
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
	b.WriteString(selicLine(r.Header) + "\n")
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
			// Só a linha que destoa do cabeçalho carrega data: repeti-la em
			// todas afogaria justamente a que o leitor precisa notar.
			if line.ReferenceAt != nil && !sameDay(*line.ReferenceAt, r.Header.ReferenceAt) {
				fmt.Fprintf(&b, " · ref %s", line.ReferenceAt.Format("02/01"))
			}
			b.WriteString("\n")
		}
	}

	if r.Bazin != nil {
		b.WriteString("\n" + bazinText(*r.Bazin))
	}

	if legend := legend(sawAbsent, sawNotApplicable, sawDerived, sawFallback, sawSmallerPeerN); legend != "" {
		fmt.Fprintf(&b, "\n%s\n", legend)
	}
	return b.String()
}

const bazinLabel = "Preço-teto (Bazin)"

func bazinText(v BazinView) string {
	// O motivo entre parênteses, e nunca depois de um travessão: "—" já é o
	// símbolo de "fonte não informa" que a legenda define.
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

	for _, y := range v.Years {
		fmt.Fprintf(&b, "  %d: %s\n", y.Year, formatValue(y.Total, domain.UnitBRL))
	}
	return b.String()
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

func selicLine(h HeaderView) string {
	if h.SelicRate == nil {
		return "Selic: desconhecida"
	}
	return fmt.Sprintf("Selic: %s (%s)",
		formatValue(*h.SelicRate, domain.UnitPercent), h.SelicAt.Format("02/01/2006"))
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

// Inverso de norm.ParseBRNumber.
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
