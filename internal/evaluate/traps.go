// Package evaluate não conhece o catálogo: o texto que precisa vir de lá chega
// pronto no Input.
package evaluate

import (
	"strings"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/norm"
)

type Status string

const (
	StatusFired         Status = "disparou"
	StatusOK            Status = "ok"
	StatusNotEvaluated  Status = "nao_avaliado"
	StatusNotApplicable Status = "nao_aplicavel"
)

// Numbers e Rule existem para que o veredito nunca apareça sozinho: sem os
// números que o produziram, o alerta é uma opinião.
type Finding struct {
	ID      string
	Status  Status
	Rule    string
	Numbers map[string]float64
	Reason  string
}

// Segment é o rótulo de taxonomia que calibra o limiar de vacância. Em FII ele
// vem do nível único do cadastro, não do campo de segmento das ações.
type Input struct {
	Class                   domain.AssetClass
	Metrics                 domain.MetricSet
	Segment                 string
	HasDetail               bool
	SelicRate               *float64
	DYPercentile            *float64
	EBITNotApplicableReason string
	// CurrentYearConcentration é o share do maior provento sobre o total
	// recebido no ano corrente. nil significa nenhum provento este ano.
	CurrentYearConcentration *float64
}

const (
	payoutLimit     = 1.0
	cheapPriceBook  = 0.8
	riskPremium     = 0.06
	assetSaleShare  = 0.15
	payoutOverFFO   = 1.0
	medianPercentil = 0.5
	// Mesmo critério de internal/bazin.Compute, duplicado porque evaluate
	// não importa bazin.
	atypicalEventShare = 0.5
)

// Fundo de papel e FoF não têm imóvel, logo não têm a pergunta da vacância.
var vacancyLimits = []struct {
	match string
	limit float64
}{
	{"LOGISTIC", 0.12},
	{"SHOPPING", 0.10},
	{"LAJE", 0.20},
	{"ESCRITORIO", 0.20},
}

const (
	notForFII   = "não se aplica a FII"
	notForStock = "não se aplica a ação"
)

// Sempre os sete resultados, na mesma ordem: omitir o alerta que não pôde ser
// avaliado faria a ausência de aviso virar aprovação.
func Detect(in Input) []Finding {
	stock := in.Class != domain.ClassFII

	return []Finding{
		classGate("ALERTA-01", stock, notForFII, func() Finding { return payoutTrap(in) }),
		classGate("ALERTA-02", stock, notForFII, func() Finding { return priceBookTrap(in) }),
		classGate("ALERTA-03", stock, notForFII, func() Finding { return profitTrap(in) }),
		classGate("ALERTA-04", !stock, notForStock, func() Finding { return assetSaleTrap(in) }),
		classGate("ALERTA-05", !stock, notForStock, func() Finding { return vacancyTrap(in) }),
		classGate("ALERTA-08", true, "", func() Finding { return dividendYieldTrap(in) }),
		classGate("ALERTA-09", stock, notForFII, func() Finding { return profitLossTrap(in) }),
	}
}

func classGate(id string, applies bool, reason string, detect func() Finding) Finding {
	if !applies {
		return Finding{ID: id, Status: StatusNotApplicable, Reason: reason}
	}
	f := detect()
	f.ID = id
	return f
}

func payoutTrap(in Input) Finding {
	payout, ok := value(in.Metrics, "payout")
	if !ok {
		return notEvaluated("payout não disponível; ele depende de DY e P/L")
	}
	return decide(payout > payoutLimit,
		"payout acima de 100%",
		map[string]float64{"payout": payout})
}

func priceBookTrap(in Input) Finding {
	if in.SelicRate == nil {
		return notEvaluated("Selic desconhecida; sem ela o retorno exigido seria inventado")
	}
	priceBook, ok1 := value(in.Metrics, "pvp")
	roe, ok2 := value(in.Metrics, "roe")
	if !ok1 || !ok2 {
		return notEvaluated("P/VP ou ROE não disponível")
	}

	required := *in.SelicRate + riskPremium
	return decide(priceBook < cheapPriceBook && roe < required,
		"P/VP abaixo de 0,80 e ROE abaixo da Selic mais 6 pontos",
		map[string]float64{"pvp": priceBook, "roe": roe, "ke": required})
}

func profitTrap(in Input) Finding {
	profit, hasProfit := value(in.Metrics, "lucro_liquido")
	operating, hasOperating := value(in.Metrics, "ebit")

	// Banco não publica EBIT nesta fonte, e isso é ausência estrutural: sem a
	// distinção, ela se leria como coleta pendente para sempre.
	if !hasOperating && in.HasDetail && in.EBITNotApplicableReason != "" {
		return Finding{Status: StatusNotApplicable, Reason: in.EBITNotApplicableReason}
	}
	if !hasProfit || !hasOperating {
		return notEvaluated("Lucro Líquido ou EBIT não coletado; rode 'goinvest detalhar'")
	}

	return decide(profit > operating,
		"Lucro Líquido maior que o EBIT",
		map[string]float64{"lucro_liquido": profit, "ebit": operating})
}

func assetSaleTrap(in Input) Finding {
	sales, ok1 := value(in.Metrics, "venda_ativos")
	revenue, ok2 := value(in.Metrics, "receita")
	paid, ok3 := value(in.Metrics, "rend_distribuido")
	funds, ok4 := value(in.Metrics, "ffo")
	if !ok1 || !ok2 || !ok3 || !ok4 {
		return notEvaluated("venda de ativos, receita, rendimento ou FFO não coletado; rode 'goinvest detalhar'")
	}
	if revenue <= 0 {
		return Finding{Status: StatusNotApplicable, Reason: "receita não positiva: a razão com a venda de ativos não tem leitura"}
	}
	if funds <= 0 {
		return Finding{Status: StatusNotApplicable, Reason: "FFO não positivo: o fundo não gerou caixa operacional, e a razão com o rendimento inverteria a leitura"}
	}

	saleShare := sales / revenue
	payoutShare := paid / funds
	return decide(saleShare > assetSaleShare && payoutShare > payoutOverFFO,
		"venda de ativos acima de 15% da receita e rendimento distribuído acima do FFO",
		map[string]float64{
			"venda_sobre_receita":  saleShare,
			"rendimento_sobre_ffo": payoutShare,
		})
}

func vacancyTrap(in Input) Finding {
	limit, known := vacancyLimit(in.Segment)
	if !known {
		return Finding{
			Status: StatusNotApplicable,
			Reason: "vacância não calibrada para este segmento; a faixa conhecida cobre logística, shoppings e lajes corporativas",
		}
	}
	vacancy, ok := value(in.Metrics, "vacancia_media")
	if !ok {
		return notEvaluated("vacância média não disponível")
	}
	if in.DYPercentile == nil {
		return notEvaluated("sem referência de DY do segmento; a amostra não permite comparar com a mediana")
	}

	return decide(vacancy > limit && *in.DYPercentile > medianPercentil,
		"vacância acima do limiar do segmento e DY acima da mediana do segmento",
		map[string]float64{
			"vacancia_media": vacancy,
			"limiar":         limit,
			"dy_percentil":   *in.DYPercentile,
		})
}

func vacancyLimit(segment string) (float64, bool) {
	folded := norm.FoldUpper(segment)
	for _, l := range vacancyLimits {
		if strings.Contains(folded, l.match) {
			return l.limit, true
		}
	}
	return 0, false
}

func dividendYieldTrap(in Input) Finding {
	if in.CurrentYearConcentration == nil {
		return notEvaluated("nenhum provento no ano corrente; sem sinal de concentração")
	}
	share := *in.CurrentYearConcentration
	return decide(share > atypicalEventShare,
		"um provento isolado responde por mais da metade do recebido no ano corrente",
		map[string]float64{"concentracao_ano_corrente": share})
}

func profitLossTrap(in Input) Finding {
	pl, ok := value(in.Metrics, "pl")
	if !ok {
		return notEvaluated("P/L não disponível")
	}
	return decide(pl < 0,
		"P/L negativo: a empresa reportou prejuízo no período",
		map[string]float64{"pl": pl})
}

func decide(fired bool, rule string, numbers map[string]float64) Finding {
	status := StatusOK
	if fired {
		status = StatusFired
	}
	return Finding{Status: status, Rule: rule, Numbers: numbers}
}

func notEvaluated(reason string) Finding {
	return Finding{Status: StatusNotEvaluated, Reason: reason}
}

func value(m domain.MetricSet, id domain.MetricID) (float64, bool) {
	o, present := m[id]
	if !present || o.Value == nil || norm.IsAbsenceSentinel(id, *o.Value) {
		return 0, false
	}
	return *o.Value, true
}
