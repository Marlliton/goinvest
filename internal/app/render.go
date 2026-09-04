package app

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/marlliton/goinvest/internal/domain"
)

const (
	markAbsent   = "—"
	markDerived  = "ƒ"
	markFallback = "*"
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
	if r.Header.IncompleteRegistry > 0 {
		fmt.Fprintf(&b, "cadastro incompleto: %d de %d\n",
			r.Header.IncompleteRegistry, r.Header.TotalInClass)
	}

	if r.Header.PeerGroupLabel != "" {
		fmt.Fprintf(&b, "Comparado com %s (%d papéis líquidos)\n",
			r.Header.PeerGroupLabel, r.Header.PeerGroupN)
	}

	sawAbsent, sawDerived, sawFallback := false, false, false
	for _, block := range r.Blocks {
		fmt.Fprintf(&b, "\n%s\n", block.Label)
		for _, line := range block.Lines {
			b.WriteString("  " + line.Label + ": ")
			if line.Value == nil {
				sawAbsent = true
				b.WriteString(markAbsent + "\n")
				continue
			}
			b.WriteString(formatValue(*line.Value, line.Unit))
			if line.Percentile != nil {
				fmt.Fprintf(&b, " · p%d · n=%d", int(*line.Percentile*100), *line.PeerN)
				if line.FellBackToMarket {
					sawFallback = true
					b.WriteString(" " + markFallback)
				}
			}
			if line.Derived {
				sawDerived = true
				fmt.Fprintf(&b, " %s (%s)", markDerived, line.Formula)
			}
			b.WriteString("\n")
		}
	}

	if legend := legend(sawAbsent, sawDerived, sawFallback); legend != "" {
		fmt.Fprintf(&b, "\n%s\n", legend)
	}
	return b.String()
}

func legend(sawAbsent, sawDerived, sawFallback bool) string {
	var parts []string
	if sawAbsent {
		parts = append(parts, markAbsent+" = fonte não informa")
	}
	if sawDerived {
		parts = append(parts, markDerived+" = calculado por goinvest")
	}
	if sawFallback {
		parts = append(parts, markFallback+" = comparado com o mercado inteiro; o setor tem poucos papéis com esta métrica")
	}
	return strings.Join(parts, " · ")
}

func sectorLine(h HeaderView) string {
	if h.Sector == "" {
		return "Setor: desconhecido"
	}
	return fmt.Sprintf("Setor: %s / %s / %s", h.Sector, h.Subsector, h.Segment)
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
			b.WriteString("  " + sectorGroupLine(s) + "\n")
		}
	}
	return b.String()
}

func RenderSectorsDescend(sector string, groups []SectorGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s · subsetores\n", sector)
	for _, s := range groups {
		b.WriteString("  " + sectorGroupLine(s) + "\n")
	}
	return b.String()
}

func sectionLabel(c domain.AssetClass) string {
	if c == domain.ClassFII {
		return "FIIs"
	}
	return "Ações"
}

func sectorGroupLine(s SectorGroup) string {
	if s.BelowThreshold {
		return fmt.Sprintf("%s — %s: percentil cai para a referência de mercado",
			s.Name, plural(s.N, "papel líquido", "papéis líquidos"))
	}
	return fmt.Sprintf("%s — %s", s.Name, plural(s.N, "ativo líquido", "ativos líquidos"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
