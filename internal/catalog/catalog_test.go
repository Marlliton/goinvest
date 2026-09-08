package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

func TestLoadEmbeddedCatalog(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)
	require.Len(t, c.Blocks, 5)
	require.Len(t, c.Metrics, 38)
	require.Len(t, c.Glossary, 38)
}

// Chave faltando aqui apagaria o texto de um alerta sem nenhum erro: o
// detector continua disparando, e só a explicação some da tela.
func TestLoadEmbeddedTrapText(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	for _, id := range []string{"ALERTA-01", "ALERTA-02", "ALERTA-03", "ALERTA-04", "ALERTA-05"} {
		_, ok := c.TrapText[id]
		require.True(t, ok, "alerta %s sem entrada em traps.yaml", id)
	}
	require.Len(t, c.TrapText, 5)
}

func TestCatalogMetricLookup(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	ebit, ok := c.Metric("ebit")
	require.True(t, ok)
	require.NotEmpty(t, ebit.NotApplicable["setor"])

	_, ok = c.Metric("nao_existe")
	require.False(t, ok)
}

func TestLoadRejectsUnknownInput(t *testing.T) {
	_, err := loadFrom(fixture(t, "unknown-input.metrics.yaml"), fixture(t, "valid.glossary.yaml"), trapsYAML)
	require.ErrorContains(t, err, "dy_typo")
}

func TestLoadRejectsMissingGlossaryEntry(t *testing.T) {
	_, err := loadFrom(fixture(t, "valid.metrics.yaml"), fixture(t, "incomplete.glossary.yaml"), trapsYAML)
	require.ErrorContains(t, err, "glossary")
}

func TestLoadRejectsDerivedWithoutFormula(t *testing.T) {
	_, err := loadFrom(fixture(t, "derived-no-formula.metrics.yaml"), fixture(t, "valid.glossary.yaml"), trapsYAML)
	require.ErrorContains(t, err, "payout")
}

// O campo existe para forçar a pergunta "quando este número engana?" no
// momento em que a métrica nasce. Sem a regra de carga, o campo vira opcional
// na prática e o próximo derivado entra sem ela.
func TestLoadRejectsDerivedWithoutNotApplicable(t *testing.T) {
	_, err := loadFrom(fixture(t, "derived-no-not-applicable.metrics.yaml"), fixture(t, "valid.glossary.yaml"), trapsYAML)
	require.ErrorContains(t, err, "does not declare when it does not apply")
}

// Sem o motivo de setor a tela imprimiria "não se aplica" sem dizer por quê, e
// motivo de outra origem não serve de substituto.
func TestLoad_SentinelWithoutSetorReason(t *testing.T) {
	_, err := loadFrom(fixture(t, "sentinel-no-setor.metrics.yaml"), fixture(t, "valid.glossary.yaml"), trapsYAML)
	require.ErrorContains(t, err, "no not_applicable[setor] reason")
}

func TestEveryDerivedMetricDeclaresWhenItDoesNotApply(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	derived := 0
	for _, m := range c.Metrics {
		if !m.Derived {
			continue
		}
		derived++
		require.NotEmpty(t, m.NotApplicable["ativo"], "métrica derivada %q", m.ID)
	}
	require.Equal(t, 3, derived)
}

func TestMetricsForExcludesOtherClasses(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	for _, m := range c.MetricsFor(domain.ClassFII) {
		require.NotEqual(t, domain.MetricID("pl"), m.ID, "P/L não existe para FII")
	}
	require.NotEmpty(t, c.MetricsFor(domain.ClassFII))
	require.NotEmpty(t, c.MetricsFor(domain.ClassStock))
}

func TestBlocksOrderedSortsByOrder(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)
	require.Equal(t,
		[]string{"cotacao", "valuation", "rentabilidade", "endividamento", "dividendos"},
		blockIDs(c.BlocksOrdered()))

	shuffled, err := loadFrom(fixture(t, "shuffled-blocks.metrics.yaml"), fixture(t, "valid.glossary.yaml"), trapsYAML)
	require.NoError(t, err)
	require.Equal(t, []string{"b1", "b2"}, blockIDs(shuffled.BlocksOrdered()))
}

func blockIDs(blocks []Block) []string {
	ids := make([]string, len(blocks))
	for i, b := range blocks {
		ids[i] = b.ID
	}
	return ids
}

func TestPercentileDeclarations(t *testing.T) {
	cat, err := Load()
	require.NoError(t, err)

	byID := make(map[domain.MetricID]Metric, len(cat.Metrics))
	for _, m := range cat.Metrics {
		byID[m.ID] = m
	}

	require.True(t, byID["pl"].Percentile)
	require.True(t, byID["pl"].ExcludeNegative, "P/L negativo não tem lugar na distribuição")
	require.True(t, byID["pvp"].ExcludeNegative)

	require.False(t, byID["cotacao"].Percentile)
	require.False(t, byID["liq_2meses"].Percentile)
	require.False(t, byID["dl_ebitda"].Percentile)

	require.Contains(t, byID["psr"].SentinelSegments, "Bancos")
	require.True(t, byID["pvp"].NegativeEquityCheck)
	require.NotEmpty(t, byID["pvp"].NotApplicable["ativo"])
	require.Contains(t, byID["ev_ebitda"].SentinelSegments, "Bancos")
	require.Empty(t, byID["pl"].SentinelSegments, "P/L de banco é número real, não sentinela")
	require.Empty(t, byID["dy"].SentinelSegments)
}

// Percentil sobre valor absoluto ordenaria empresa por tamanho, não por
// qualidade.
func TestDetailMetricsAreDeclaredWithoutPercentile(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	byClass := map[domain.AssetClass][]domain.MetricID{
		domain.ClassStock: {"lucro_liquido", "ebit"},
		domain.ClassFII:   {"venda_ativos", "ffo", "rend_distribuido", "receita"},
	}
	for class, want := range byClass {
		byID := make(map[domain.MetricID]Metric)
		for _, m := range c.MetricsFor(class) {
			byID[m.ID] = m
		}
		for _, id := range want {
			m, ok := byID[id]
			require.True(t, ok, "métrica %q ausente para a classe %s", id, class)
			require.Equal(t, domain.UnitBRL, m.Unit, "métrica %q", id)
			require.False(t, m.Percentile, "métrica %q", id)
		}
	}
}

// EBIT nunca aparece como 0,00 para banco: o rótulo não existe na página. A
// sentinela por segmento continua servindo porque a consequência é a mesma, a
// métrica sai da distribuição do setor.
func TestEbitCarriesSectorReason(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	for _, m := range c.Metrics {
		if m.ID != "ebit" {
			continue
		}
		require.Contains(t, m.SentinelSegments, "Bancos")
		require.Contains(t, m.NotApplicable["setor"], "Banco")
		return
	}
	t.Fatal("métrica ebit ausente do catálogo")
}

// A regra só tem dente se valer para toda métrica com sentinela, não só para a
// que a motivou.
func TestEverySentinelMetricExplainsTheSector(t *testing.T) {
	c, err := Load()
	require.NoError(t, err)

	sentinels := 0
	for _, m := range c.Metrics {
		if len(m.SentinelSegments) == 0 {
			continue
		}
		sentinels++
		require.NotEmpty(t, m.NotApplicable["setor"], "métrica %q", m.ID)
	}
	require.Equal(t, 14, sentinels)
}
