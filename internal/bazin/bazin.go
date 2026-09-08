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

	minYears    = 3
	windowYears = 5
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

	years := slices.SortedFunc(maps.Keys(totals), func(a, b int) int { return b - a })
	if len(years) > windowYears {
		years = years[:windowYears]
	}
	if len(years) < minYears {
		return Result{}, false
	}

	out := Result{YearsAvailable: len(years)}
	for _, year := range years {
		total := totals[year]
		out.Years = append(out.Years, YearlyDividend{Year: year, Total: total})
		out.AverageAnnual += total
		if total > 0 && largest[year]/total > atypicalEventShare {
			out.AtypicalYears = append(out.AtypicalYears, year)
		}
	}
	out.AverageAnnual /= float64(len(years))
	out.Ceiling = out.AverageAnnual / targetYield
	if !finite(out.AverageAnnual, out.Ceiling) {
		return Result{}, false
	}
	return out, true
}

func netPerShare(e domain.DividendEvent) (float64, bool) {
	if e.SharesFactor <= 0 {
		return 0, false
	}
	v := e.ValuePerShareRaw / e.SharesFactor
	if e.Type == domain.DividendJCP {
		v *= jcpNetOfTax
	}
	if !finite(v) {
		return 0, false
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
