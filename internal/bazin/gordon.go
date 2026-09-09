package bazin

// riskPremium duplica internal/evaluate/traps.go (mesmo Ke usado no alerta de
// P/VP contra ROE). bazin não pode importar evaluate; qualquer mudança aqui
// precisa espelhar lá.
const riskPremium = 0.06

func RequiredReturn(selicRate float64) float64 { return selicRate + riskPremium }

type GordonResult struct {
	Ceiling float64
	Ke      float64
	G       float64
}

// Gordon estoura quando g >= Ke: o denominador deixa de ser positivo.
func Gordon(dividendPerShare, requiredReturn, growth float64) (GordonResult, bool) {
	if growth >= requiredReturn {
		return GordonResult{}, false
	}
	ceiling := dividendPerShare / (requiredReturn - growth)
	if !finite(ceiling) || ceiling <= 0 {
		return GordonResult{}, false
	}
	return GordonResult{Ceiling: ceiling, Ke: requiredReturn, G: growth}, true
}
