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
	require.Len(t, c.Metrics, 32)
	require.Len(t, c.Glossary, 32)
}

func TestLoadRejectsUnknownInput(t *testing.T) {
	_, err := loadFrom(fixture(t, "unknown-input.metrics.yaml"), fixture(t, "valid.glossary.yaml"))
	require.ErrorContains(t, err, "dy_typo")
}

func TestLoadRejectsMissingGlossaryEntry(t *testing.T) {
	_, err := loadFrom(fixture(t, "valid.metrics.yaml"), fixture(t, "incomplete.glossary.yaml"))
	require.ErrorContains(t, err, "glossary")
}

func TestLoadRejectsDerivedWithoutFormula(t *testing.T) {
	_, err := loadFrom(fixture(t, "derived-no-formula.metrics.yaml"), fixture(t, "valid.glossary.yaml"))
	require.ErrorContains(t, err, "payout")
}

// O campo existe para forçar a pergunta "quando este número engana?" no
// momento em que a métrica nasce. Sem a regra de carga, o campo vira opcional
// na prática e o próximo derivado entra sem ela.
func TestLoadRejectsDerivedWithoutNotApplicable(t *testing.T) {
	_, err := loadFrom(fixture(t, "derived-no-not-applicable.metrics.yaml"), fixture(t, "valid.glossary.yaml"))
	require.ErrorContains(t, err, "does not declare when it does not apply")
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
		require.NotEmpty(t, m.NotApplicable, "métrica derivada %q", m.ID)
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

	shuffled, err := loadFrom(fixture(t, "shuffled-blocks.metrics.yaml"), fixture(t, "valid.glossary.yaml"))
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

// Percentil de cotação não significa nada: o preço unitário depende do tamanho
// do lote, não de o papel estar caro ou barato.
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
	// Derivado não é gravado em observation, então não entra na materialização.
	require.False(t, byID["dl_ebitda"].Percentile)

	require.Contains(t, byID["psr"].SentinelSegments, "Bancos")
	require.Contains(t, byID["ev_ebitda"].SentinelSegments, "Bancos")
	require.Empty(t, byID["pl"].SentinelSegments, "P/L de banco é número real, não sentinela")
	require.Empty(t, byID["dy"].SentinelSegments)
}
