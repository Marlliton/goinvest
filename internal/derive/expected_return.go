package derive

import "github.com/marlliton/goinvest/internal/domain"

// Decomposition abre o retorno esperado de uma ação nas parcelas que o compõem.
type Decomposition struct {
	EarningsYield float64 // 1 / P/L: lucro sobre o preço
	Distributed   float64 // DY: parcela do lucro que vira dividendo
	Retained      float64 // 1 - payout: parcela retida
	ImpliedGrowth float64 // g = ROE × retenção (Damodaran, crescimento sustentável)
	TotalReturn   float64 // DY + g: Gordon invertido, retorno esperado total
	PaybackYears  float64 // P/L lido como tempo: anos de lucro para pagar o preço de hoje
}

// Decompose exige o MetricSet já mesclado com os derivados (o merged que
// internal/app monta com derive.Compute(collected) + collected): payout só
// existe depois desse merge, nunca em collected puro.
func Decompose(m domain.MetricSet) (Decomposition, bool) {
	pl, ok1 := value(m, "pl")
	dy, ok2 := value(m, "dy")
	roe, ok3 := value(m, "roe")
	payout, ok4 := value(m, "payout")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return Decomposition{}, false
	}
	// Sem lucro não há retorno esperado calculável sobre ele.
	if pl <= 0 {
		return Decomposition{}, false
	}
	// g = b × ROE pressupõe retenção entre 0 e 1 (Damodaran, "Estimating Growth").
	// Distribuir mais do que se lucrou torna b negativo, e o modelo passa a projetar
	// encolhimento perpétuo: calculável, mas sem significado.
	if payout > 1 {
		return Decomposition{}, false
	}

	retained := 1 - payout
	growth := roe * retained
	earningsYield := 1 / pl
	total := dy + growth
	if !finite(earningsYield, retained, growth, total) {
		return Decomposition{}, false
	}
	return Decomposition{earningsYield, dy, retained, growth, total, pl}, true
}
