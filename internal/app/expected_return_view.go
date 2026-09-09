package app

import (
	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
)

type ExpectedReturnView struct {
	EarningsYield       float64
	Distributed         float64
	Retained            float64
	ImpliedGrowth       float64
	TotalReturn         float64
	PaybackYears        float64
	NotApplicableReason string
}

const plID = domain.MetricID("pl")

func expectedReturnView(class domain.AssetClass, merged domain.MetricSet) *ExpectedReturnView {
	if class == domain.ClassFII {
		return &ExpectedReturnView{
			NotApplicableReason: "FII distribui por obrigação legal e não retém lucro: retenção e crescimento implícito não têm o que medir",
		}
	}

	dec, ok := derive.Decompose(merged)
	if ok {
		return &ExpectedReturnView{
			EarningsYield: dec.EarningsYield,
			Distributed:   dec.Distributed,
			Retained:      dec.Retained,
			ImpliedGrowth: dec.ImpliedGrowth,
			TotalReturn:   dec.TotalReturn,
			PaybackYears:  dec.PaybackYears,
		}
	}

	if pl, has := presentValue(merged, plID); has && pl <= 0 {
		return &ExpectedReturnView{NotApplicableReason: "Prejuízo no período: não há lucro para decompor em retorno esperado"}
	}
	return &ExpectedReturnView{NotApplicableReason: "P/L, DY, ROE ou payout não disponível"}
}
