package app

import (
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/config"
	"github.com/marlliton/goinvest/internal/domain"
)

type AnchorView struct {
	ID                 string
	Label              string
	RateGross          *float64
	ReferenceAt        *time.Time
	RateNet            *float64
	SpreadNet          *float64
	NotEvaluatedReason string
}

type TaxAssumptionView struct {
	AliquotaAtivo     float64
	AliquotaRendaFixa float64
	ConfigPath        string
	FromFile          bool
}

type OpportunityCostView struct {
	Anchors                []AnchorView
	TaxAssumption          TaxAssumptionView
	IPCANominalizationNote string
}

type macroRate struct {
	Rate        *float64
	ReferenceAt *time.Time
}

func opportunityCostView(selic, cdi, ipca10y, focus macroRate, distributed, growth *float64, tax config.Result) OpportunityCostView {
	var netTotal *float64
	if distributed != nil {
		v := *distributed * (1 - tax.Tributacao.AliquotaAtivo)
		if growth != nil {
			v += *growth
		}
		netTotal = &v
	}

	ipcaNominal := macroRate{ReferenceAt: ipca10y.ReferenceAt}
	if ipca10y.Rate != nil && focus.Rate != nil {
		v := *ipca10y.Rate + *focus.Rate
		ipcaNominal.Rate = &v
	}
	tesouro := anchorLine("tesouro_ipca_10a", "Tesouro IPCA+ (~10 anos)", ipcaNominal, netTotal, tax)
	if ipca10y.Rate != nil && focus.Rate == nil {
		tesouro.NotEvaluatedReason = "Tesouro IPCA+ desconhecido: falta a expectativa de inflação (Focus) para nominalizar"
	}

	return OpportunityCostView{
		Anchors: []AnchorView{
			anchorLine("selic", "Selic", selic, netTotal, tax),
			anchorLine("cdi", "CDI", cdi, netTotal, tax),
			tesouro,
		},
		TaxAssumption: TaxAssumptionView{
			AliquotaAtivo:     tax.Tributacao.AliquotaAtivo,
			AliquotaRendaFixa: tax.Tributacao.AliquotaRendaFixa,
			ConfigPath:        tax.Path,
			FromFile:          tax.FromFile,
		},
		IPCANominalizationNote: ipcaNominalizationNote(ipca10y, focus),
	}
}

func anchorLine(id, label string, r macroRate, netTotal *float64, tax config.Result) AnchorView {
	if r.Rate == nil {
		return AnchorView{ID: id, Label: label, NotEvaluatedReason: label + " desconhecida; rode 'goinvest sync'"}
	}
	net := *r.Rate * (1 - tax.Tributacao.AliquotaRendaFixa)
	v := AnchorView{ID: id, Label: label, RateGross: r.Rate, ReferenceAt: r.ReferenceAt, RateNet: &net}
	if netTotal != nil {
		spread := *netTotal - net
		v.SpreadNet = &spread
	}
	return v
}

func ipcaNominalizationNote(ipca10y, focus macroRate) string {
	if ipca10y.Rate == nil || focus.Rate == nil {
		return ""
	}
	nominal := *ipca10y.Rate + *focus.Rate
	return fmt.Sprintf("IPCA+ %s com inflação esperada de %s = %s nominal",
		formatValue(*ipca10y.Rate, domain.UnitPercent),
		formatValue(*focus.Rate, domain.UnitPercent),
		formatValue(nominal, domain.UnitPercent))
}
