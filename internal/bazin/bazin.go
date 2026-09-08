// Package bazin calcula o preço-teto do método de Décio Bazin a partir da
// série de proventos. Nada aqui toca rede, banco ou catálogo.
package bazin

import (
	"maps"
	"math"
	"slices"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
)

const (
	// O rendimento mínimo exigido pelo método.
	targetYield = 0.06
	// JCP é tributado em 15% retidos na fonte: o que entra na média é o que o
	// investidor pessoa física de fato recebe.
	jcpNetOfTax = 0.85
	// Um provento que sozinho responde por mais da metade do total pago naquele
	// ano é considerado concentração atípica.
	atypicalEventShare = 0.5

	// MinYears é o piso: abaixo dele a média não suaviza nada.
	MinYears = 3
	// WindowYears é a janela do método: cinco exercícios fechados.
	WindowYears = 5
)

type YearlyDividend struct {
	Year  int
	Total float64
}

type Result struct {
	Ceiling        float64
	AverageAnnual  float64
	Years          []YearlyDividend
	YearsAvailable int
	AtypicalYears  []int
	// Exercícios da janela em que o ativo não pagou nada. Preenchido junto com
	// ok == false: o método é declaradamente inaplicável a quem não paga com
	// consistência, e a média sobre os anos restantes sairia inflada.
	MissingYears []int
}

// Compute agrupa os eventos por ano de data-com, usa até cinco exercícios
// fechados e devolve false quando o piso de três anos não é atingido: uma média
// de um ou dois anos não suaviza distribuição extraordinária nenhuma, que é
// justamente o que a média existe para fazer.
func Compute(events []domain.DividendEvent, now time.Time) (Result, bool) {
	totals := map[int]float64{}
	largest := map[int]float64{}

	for _, e := range events {
		if e.ExDate.Year() >= now.Year() {
			continue
		}
		v, ok := netPerShare(e)
		if !ok {
			continue
		}
		year := e.ExDate.Year()
		totals[year] += v
		largest[year] = math.Max(largest[year], v)
	}
	if len(totals) == 0 {
		return Result{}, false
	}

	// A janela é contígua e termina no último exercício fechado: contar só os
	// anos que têm linha faria um ano sem pagamento sumir da conta.
	latest := now.Year() - 1
	first := max(slices.Min(slices.Collect(maps.Keys(totals))), latest-WindowYears+1)

	out := Result{YearsAvailable: latest - first + 1}
	for year := latest; year >= first; year-- {
		total, paid := totals[year]
		if !paid {
			out.MissingYears = append(out.MissingYears, year)
			continue
		}
		out.Years = append(out.Years, YearlyDividend{Year: year, Total: total})
		out.AverageAnnual += total
		if total > 0 && largest[year]/total > atypicalEventShare {
			out.AtypicalYears = append(out.AtypicalYears, year)
		}
	}
	if len(out.MissingYears) > 0 {
		return Result{MissingYears: out.MissingYears}, false
	}
	if out.YearsAvailable < MinYears {
		return Result{YearsAvailable: out.YearsAvailable}, false
	}

	out.AverageAnnual /= float64(out.YearsAvailable)
	out.Ceiling = out.AverageAnnual / targetYield
	if !finite(out.AverageAnnual, out.Ceiling) {
		return Result{}, false
	}
	return out, true
}

func netPerShare(e domain.DividendEvent) (float64, bool) {
	v, ok := e.PerShare()
	if !ok {
		return 0, false
	}
	if e.Type == domain.DividendJCP {
		v *= jcpNetOfTax
	}
	return v, true
}

func finite(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}
