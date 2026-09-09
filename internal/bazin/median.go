package bazin

import "slices"

// MedianDividendYield usa a mesma série de exercícios fechados que Compute já
// calculou, dividida pela cotação de hoje: a pergunta é "quanto os últimos
// cinco anos renderiam sobre o preço que eu pagaria agora", não uma série de
// DY histórica com o preço de cada época.
func MedianDividendYield(result Result, currentPrice float64) (float64, bool) {
	if currentPrice <= 0 || len(result.Years) == 0 {
		return 0, false
	}
	yields := make([]float64, 0, len(result.Years))
	for _, y := range result.Years {
		yields = append(yields, y.Total/currentPrice)
	}
	slices.Sort(yields)
	n := len(yields)
	median := yields[n/2]
	if n%2 == 0 {
		median = (yields[n/2-1] + yields[n/2]) / 2
	}
	if !finite(median) {
		return 0, false
	}
	return median, true
}
