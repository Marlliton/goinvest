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
