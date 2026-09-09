package evaluate_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
	"github.com/stretchr/testify/require"
)

func ptr(v float64) *float64 { return &v }

func metrics(values map[domain.MetricID]float64) domain.MetricSet {
	set := domain.MetricSet{}
	for id, v := range values {
		set[id] = domain.Observation{Metric: id, Value: ptr(v)}
	}
	return set
}

func findingOf(t *testing.T, findings []evaluate.Finding, id string) evaluate.Finding {
	t.Helper()
	for _, f := range findings {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("alerta %s ausente da lista", id)
	return evaluate.Finding{}
}

func stock(values map[domain.MetricID]float64) evaluate.Input {
	return evaluate.Input{Class: domain.ClassStock, Metrics: metrics(values)}
}

func fii(segment string, values map[domain.MetricID]float64) evaluate.Input {
	return evaluate.Input{Class: domain.ClassFII, Segment: segment, Metrics: metrics(values)}
}

func TestDetect_Payout(t *testing.T) {
	fired := findingOf(t, evaluate.Detect(stock(map[domain.MetricID]float64{"payout": 1.23})), "ALERTA-01")
	require.Equal(t, evaluate.StatusFired, fired.Status)
	require.InDelta(t, 1.23, fired.Numbers["payout"], 1e-9)
	require.NotEmpty(t, fired.Rule)

	ok := findingOf(t, evaluate.Detect(stock(map[domain.MetricID]float64{"payout": 0.40})), "ALERTA-01")
	require.Equal(t, evaluate.StatusOK, ok.Status)
	require.InDelta(t, 0.40, ok.Numbers["payout"], 1e-9)

	absent := findingOf(t, evaluate.Detect(stock(nil)), "ALERTA-01")
	require.Equal(t, evaluate.StatusNotEvaluated, absent.Status)
	require.NotEmpty(t, absent.Reason)
}

func TestDetect_PVPROESelicUnknown(t *testing.T) {
	in := stock(map[domain.MetricID]float64{"pvp": 0.6, "roe": 0.10})
	got := findingOf(t, evaluate.Detect(in), "ALERTA-02")

	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
	require.Contains(t, got.Reason, "Selic")
}

func TestDetect_PVPROEFired(t *testing.T) {
	in := stock(map[domain.MetricID]float64{"pvp": 0.6, "roe": 0.10})
	in.SelicRate = ptr(0.14)

	got := findingOf(t, evaluate.Detect(in), "ALERTA-02")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 0.20, got.Numbers["ke"], 1e-9)
	require.InDelta(t, 0.6, got.Numbers["pvp"], 1e-9)
	require.InDelta(t, 0.10, got.Numbers["roe"], 1e-9)
}

func TestDetect_PVPROENotFiredWhenOnlyOneTermHolds(t *testing.T) {
	cheapButProfitable := stock(map[domain.MetricID]float64{"pvp": 0.6, "roe": 0.30})
	cheapButProfitable.SelicRate = ptr(0.14)
	require.Equal(t, evaluate.StatusOK,
		findingOf(t, evaluate.Detect(cheapButProfitable), "ALERTA-02").Status)

	expensiveAndWeak := stock(map[domain.MetricID]float64{"pvp": 2.0, "roe": 0.10})
	expensiveAndWeak.SelicRate = ptr(0.14)
	require.Equal(t, evaluate.StatusOK,
		findingOf(t, evaluate.Detect(expensiveAndWeak), "ALERTA-02").Status)
}

func TestDetect_ProfitBankNotApplicable(t *testing.T) {
	in := stock(map[domain.MetricID]float64{"lucro_liquido": 40e9})
	in.HasDetail = true
	in.EBITNotApplicableReason = "Banco: o EBIT não é publicado para instituições financeiras nesta fonte"

	got := findingOf(t, evaluate.Detect(in), "ALERTA-03")
	require.Equal(t, evaluate.StatusNotApplicable, got.Status)
	require.Equal(t, in.EBITNotApplicableReason, got.Reason)
}

func TestDetect_ProfitNeverCollected(t *testing.T) {
	got := findingOf(t, evaluate.Detect(stock(nil)), "ALERTA-03")
	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
}

func TestDetect_ProfitFired(t *testing.T) {
	in := stock(map[domain.MetricID]float64{"lucro_liquido": 12e9, "ebit": 9e9})
	in.HasDetail = true

	got := findingOf(t, evaluate.Detect(in), "ALERTA-03")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 12e9, got.Numbers["lucro_liquido"], 1)
	require.InDelta(t, 9e9, got.Numbers["ebit"], 1)
}

func TestDetect_AssetSaleFired(t *testing.T) {
	in := fii("Logística", map[domain.MetricID]float64{
		"venda_ativos": 80e6, "receita": 500e6, "rend_distribuido": 500e6, "ffo": 480e6,
	})
	in.HasDetail = true

	got := findingOf(t, evaluate.Detect(in), "ALERTA-04")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 0.16, got.Numbers["venda_sobre_receita"], 1e-9)
	require.InDelta(t, 500.0/480.0, got.Numbers["rendimento_sobre_ffo"], 1e-9)
}

// MXRF11 real: distribui 103% do FFO, patamar comum, mas vende só 9,7% da
// receita. Um gatilho de "venda_ativos > 0" chamaria isso de armadilha.
func TestDetect_AssetSaleNotFired_RealMXRF11(t *testing.T) {
	in := fii("Papel", map[domain.MetricID]float64{
		"venda_ativos": 51.3e6, "receita": 526.5e6, "rend_distribuido": 497.1e6, "ffo": 483.1e6,
	})
	in.HasDetail = true

	got := findingOf(t, evaluate.Detect(in), "ALERTA-04")
	require.Equal(t, evaluate.StatusOK, got.Status)
	require.Less(t, got.Numbers["venda_sobre_receita"], 0.15)
	require.Greater(t, got.Numbers["rendimento_sobre_ffo"], 1.0)
}

func TestDetect_AssetSaleNeverCollected(t *testing.T) {
	got := findingOf(t, evaluate.Detect(fii("Logística", nil)), "ALERTA-04")
	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
	require.NotEmpty(t, got.Reason)
}

// Distribuir sobre FFO negativo dá razão negativa, que a regra leria como
// "distribui menos que gera".
func TestDetect_AssetSaleNonPositiveFFO(t *testing.T) {
	in := fii("Logística", map[domain.MetricID]float64{
		"venda_ativos": 80e6, "receita": 500e6, "rend_distribuido": 500e6, "ffo": -10e6,
	})
	in.HasDetail = true

	got := findingOf(t, evaluate.Detect(in), "ALERTA-04")
	require.Equal(t, evaluate.StatusNotApplicable, got.Status)
	require.NotEmpty(t, got.Reason)
}

func TestDetect_VacancyUnknownSegment(t *testing.T) {
	in := fii("Papel", map[domain.MetricID]float64{"vacancia_media": 0.15})
	in.DYPercentile = ptr(0.7)

	got := findingOf(t, evaluate.Detect(in), "ALERTA-05")
	require.Equal(t, evaluate.StatusNotApplicable, got.Status)
	require.NotEmpty(t, got.Reason)
}

func TestDetect_VacancyFired(t *testing.T) {
	in := fii("Logística", map[domain.MetricID]float64{"vacancia_media": 0.15})
	in.DYPercentile = ptr(0.7)

	got := findingOf(t, evaluate.Detect(in), "ALERTA-05")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 0.15, got.Numbers["vacancia_media"], 1e-9)
	require.InDelta(t, 0.12, got.Numbers["limiar"], 1e-9)
}

// 15% é alerta em logística e ainda é faixa de atenção em laje corporativa,
// onde o alerta começa em 20%.
func TestDetect_VacancyThresholdVariesBySegment(t *testing.T) {
	for _, c := range []struct {
		segment string
		want    evaluate.Status
	}{
		{"Logística", evaluate.StatusFired},
		{"Shoppings", evaluate.StatusFired},
		{"Lajes Corporativas", evaluate.StatusOK},
		{"Escritórios", evaluate.StatusOK},
	} {
		t.Run(c.segment, func(t *testing.T) {
			in := fii(c.segment, map[domain.MetricID]float64{"vacancia_media": 0.15})
			in.DYPercentile = ptr(0.7)
			require.Equal(t, c.want, findingOf(t, evaluate.Detect(in), "ALERTA-05").Status)
		})
	}
}

func TestDetect_VacancyWithoutPercentile(t *testing.T) {
	in := fii("Logística", map[domain.MetricID]float64{"vacancia_media": 0.15})

	got := findingOf(t, evaluate.Detect(in), "ALERTA-05")
	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
}

// Vacância alta com DY baixo é problema já declarado no preço, não dividendo
// insustentável.
func TestDetect_VacancyBelowMedianYield(t *testing.T) {
	in := fii("Logística", map[domain.MetricID]float64{"vacancia_media": 0.15})
	in.DYPercentile = ptr(0.30)

	require.Equal(t, evaluate.StatusOK, findingOf(t, evaluate.Detect(in), "ALERTA-05").Status)
}

func TestDetect_ProfitLoss_Fires(t *testing.T) {
	got := findingOf(t, evaluate.Detect(stock(map[domain.MetricID]float64{"pl": -3.0})), "ALERTA-09")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, -3.0, got.Numbers["pl"], 1e-9)
	require.NotEmpty(t, got.Rule)
}

func TestDetect_ProfitLoss_OK(t *testing.T) {
	got := findingOf(t, evaluate.Detect(stock(map[domain.MetricID]float64{"pl": 8.0})), "ALERTA-09")
	require.Equal(t, evaluate.StatusOK, got.Status)
	require.InDelta(t, 8.0, got.Numbers["pl"], 1e-9)
}

func TestDetect_ProfitLoss_NotEvaluated(t *testing.T) {
	got := findingOf(t, evaluate.Detect(stock(nil)), "ALERTA-09")
	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
	require.NotEmpty(t, got.Reason)
}

func TestDetect_ProfitLoss_NotApplicableForFII(t *testing.T) {
	got := findingOf(t, evaluate.Detect(fii("Logística", map[domain.MetricID]float64{"pl": -3.0})), "ALERTA-09")
	require.Equal(t, evaluate.StatusNotApplicable, got.Status)
}

func TestDetect_DividendYieldSuspicion_Fires(t *testing.T) {
	in := evaluate.Input{Class: domain.ClassStock, CurrentYearConcentration: ptr(0.7)}
	got := findingOf(t, evaluate.Detect(in), "ALERTA-08")
	require.Equal(t, evaluate.StatusFired, got.Status)
	require.InDelta(t, 0.7, got.Numbers["concentracao_ano_corrente"], 1e-9)
	require.NotEmpty(t, got.Rule)
}

func TestDetect_DividendYieldSuspicion_OK(t *testing.T) {
	in := evaluate.Input{Class: domain.ClassStock, CurrentYearConcentration: ptr(0.3)}
	got := findingOf(t, evaluate.Detect(in), "ALERTA-08")
	require.Equal(t, evaluate.StatusOK, got.Status)
	require.InDelta(t, 0.3, got.Numbers["concentracao_ano_corrente"], 1e-9)
}

func TestDetect_DividendYieldSuspicion_NotEvaluated(t *testing.T) {
	got := findingOf(t, evaluate.Detect(stock(nil)), "ALERTA-08")
	require.Equal(t, evaluate.StatusNotEvaluated, got.Status)
	require.NotEmpty(t, got.Reason)
}

// FII entra na avaliação como a ação: nenhum classGate restringe o ALERTA-08.
func TestDetect_DividendYieldSuspicion_AppliesToFII(t *testing.T) {
	in := evaluate.Input{Class: domain.ClassFII, CurrentYearConcentration: ptr(0.7)}
	got := findingOf(t, evaluate.Detect(in), "ALERTA-08")
	require.NotEqual(t, evaluate.StatusNotApplicable, got.Status)
	require.Equal(t, evaluate.StatusFired, got.Status)

	inOK := evaluate.Input{Class: domain.ClassFII, CurrentYearConcentration: ptr(0.3)}
	gotOK := findingOf(t, evaluate.Detect(inOK), "ALERTA-08")
	require.NotEqual(t, evaluate.StatusNotApplicable, gotOK.Status)
	require.Equal(t, evaluate.StatusOK, gotOK.Status)
}

func TestDetect_AlwaysSevenFindings(t *testing.T) {
	for _, in := range []evaluate.Input{
		{},
		stock(nil),
		fii("Logística", nil),
		stock(map[domain.MetricID]float64{"payout": 1.23}),
	} {
		got := evaluate.Detect(in)
		require.Len(t, got, 7)
		require.Equal(t,
			[]string{"ALERTA-01", "ALERTA-02", "ALERTA-03", "ALERTA-04", "ALERTA-05", "ALERTA-08", "ALERTA-09"},
			ids(got), "a ordem é parte do contrato")
		for _, f := range got {
			require.NotEmpty(t, f.Status)
			if f.Status == evaluate.StatusNotEvaluated || f.Status == evaluate.StatusNotApplicable {
				require.NotEmpty(t, f.Reason, "%s sem motivo", f.ID)
			} else {
				require.NotEmpty(t, f.Rule, "%s sem regra", f.ID)
			}
		}
	}
}

// Sumir da lista faria o usuário concluir que passou nos cinco testes quando
// dois nem rodaram.
func TestDetect_CrossClassIsNotApplicable(t *testing.T) {
	forFII := evaluate.Detect(fii("Logística", nil))
	for _, id := range []string{"ALERTA-01", "ALERTA-02", "ALERTA-03"} {
		require.Equal(t, evaluate.StatusNotApplicable, findingOf(t, forFII, id).Status, id)
	}

	forStock := evaluate.Detect(stock(nil))
	for _, id := range []string{"ALERTA-04", "ALERTA-05"} {
		require.Equal(t, evaluate.StatusNotApplicable, findingOf(t, forStock, id).Status, id)
	}
}

func ids(findings []evaluate.Finding) []string {
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.ID)
	}
	return out
}
