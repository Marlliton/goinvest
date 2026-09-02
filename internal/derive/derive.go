// Package derive calcula na leitura os indicadores que nenhuma fonte gratuita
// publica. Nada aqui é persistido, e nada aqui toca rede, banco ou catálogo.
package derive

import (
	"math"

	"github.com/marlliton/goinvest/internal/domain"
)

// Compute devolve só os derivados que puderam ser calculados. Derivado com
// insumo ausente ou suspeito não vira chave: um número calculado sobre
// sentinela tem a mesma autoridade visual dos outros e é pior que a ausência.
func Compute(metrics domain.MetricSet) domain.MetricSet {
	out := domain.MetricSet{}
	for _, d := range derivations {
		if o, ok := d.compute(metrics); ok {
			out[o.Metric] = o
		}
	}
	return out
}

type derivation struct {
	compute func(domain.MetricSet) (domain.Observation, bool)
}

var derivations = []derivation{
	{compute: payout},
	{compute: netDebtOverEBITDA},
	{compute: returnOnAssets},
}

func payout(m domain.MetricSet) (domain.Observation, bool) {
	inputs := []domain.MetricID{"dy", "pl"}
	dy, ok1 := value(m, "dy")
	pl, ok2 := value(m, "pl")
	if !ok1 || !ok2 {
		return domain.Observation{}, false
	}
	// Sem lucro não há fatia do lucro a distribuir: a pergunta que payout
	// responde deixa de existir, e o número negativo se lê como se existisse.
	if pl <= 0 {
		return domain.Observation{}, false
	}
	return observation(m, "payout", domain.UnitPercent, dy*pl, inputs)
}

func netDebtOverEBITDA(m domain.MetricSet) (domain.Observation, bool) {
	inputs := []domain.MetricID{"pvp", "patrim_liq", "dl_patrim", "ev_ebitda"}
	pvp, ok1 := value(m, "pvp")
	equity, ok2 := value(m, "patrim_liq")
	dlEquity, ok3 := value(m, "dl_patrim")
	evEBITDA, ok4 := value(m, "ev_ebitda")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return domain.Observation{}, false
	}

	marketCap := pvp * equity
	netDebt := dlEquity * equity
	enterpriseValue := marketCap + netDebt
	ebitda := enterpriseValue / evEBITDA
	if !finite(marketCap, netDebt, enterpriseValue, ebitda) {
		return domain.Observation{}, false
	}
	// Um resultado negativo tem duas causas opostas: dívida líquida negativa
	// (a empresa tem mais caixa que dívida, notícia boa) ou EBITDA negativo
	// (prejuízo operacional). O número sozinho não distingue as duas, então só
	// a primeira sai daqui.
	if ebitda <= 0 {
		return domain.Observation{}, false
	}

	return observation(m, "dl_ebitda", domain.UnitRatio, netDebt/ebitda, inputs)
}

func returnOnAssets(m domain.MetricSet) (domain.Observation, bool) {
	inputs := []domain.MetricID{"roe", "p_ativo", "pvp", "patrim_liq"}
	roe, ok1 := value(m, "roe")
	pAsset, ok2 := value(m, "p_ativo")
	pvp, ok3 := value(m, "pvp")
	equity, ok4 := value(m, "patrim_liq")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return domain.Observation{}, false
	}

	marketCap := pvp * equity
	totalAssets := marketCap / pAsset
	netIncome := roe * equity
	if !finite(marketCap, totalAssets, netIncome) {
		return domain.Observation{}, false
	}

	return observation(m, "roa", domain.UnitPercent, netIncome/totalAssets, inputs)
}

// Checar só o resultado final não basta: um intermediário infinito volta a
// ser finito na divisão seguinte, e o derivado sai como um zero plausível.
func finite(xs ...float64) bool {
	for _, x := range xs {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return false
		}
	}
	return true
}

func value(m domain.MetricSet, id domain.MetricID) (float64, bool) {
	o, present := m[id]
	if !present || isInputSuspect(id, o.Value) {
		return 0, false
	}
	return *o.Value, true
}

// O derivado herda a proveniência dos insumos, e a data de coleta do insumo
// mais velho: ele não é mais fresco que o pior número que o produziu.
func observation(m domain.MetricSet, id domain.MetricID, unit domain.Unit, v float64, inputs []domain.MetricID) (domain.Observation, bool) {
	if !finite(v) {
		return domain.Observation{}, false
	}

	o := domain.Observation{
		Metric: id,
		Value:  &v,
		Unit:   unit,
		Source: "derive:" + string(id),
	}
	for _, in := range inputs {
		src := m[in]
		if o.Ticker == "" {
			o.Ticker = src.Ticker
			o.PeriodKind = src.PeriodKind
			o.PeriodEnd = src.PeriodEnd
			o.FetchedAt = src.FetchedAt
			continue
		}
		if src.FetchedAt.Before(o.FetchedAt) {
			o.FetchedAt = src.FetchedAt
		}
	}
	return o, true
}
