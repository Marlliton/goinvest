package fundamentus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/norm"
	"github.com/marlliton/goinvest/internal/provider/fundamentus"
	"github.com/stretchr/testify/require"
)

const testRateEvery = time.Millisecond

var fixtures = map[string]string{
	"/resultado.php":     "resultado_min.html",
	"/fii_resultado.php": "fii_resultado_min.html",
}

// O servidor devolve os bytes ISO-8859-1 da fixture sem tocar neles, com o
// mesmo Content-Type da fonte real: é o que faz o teste exercitar a
// decodificação de ponta a ponta em vez de fingi-la.
func newProvider(t *testing.T) *fundamentus.Provider {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := fixtures[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	client := fetch.NewClient(fetch.Config{RateEvery: testRateEvery})
	return fundamentus.NewProvider(client, srv.URL, time.Now)
}

func universe(t *testing.T, class domain.AssetClass) []domain.Observation {
	t.Helper()
	obs, err := newProvider(t).Universe(context.Background(), class, false)
	require.NoError(t, err)
	require.NotEmpty(t, obs)
	return obs
}

func byTicker(obs []domain.Observation, ticker string) domain.MetricSet {
	set := domain.MetricSet{}
	for _, o := range obs {
		if o.Ticker == ticker {
			set[o.Metric] = o
		}
	}
	return set
}

func metricIDs(obs []domain.Observation) []domain.MetricID {
	seen := map[domain.MetricID]struct{}{}
	out := []domain.MetricID{}
	for _, o := range obs {
		if _, dup := seen[o.Metric]; dup {
			continue
		}
		seen[o.Metric] = struct{}{}
		out = append(out, o.Metric)
	}
	return out
}

// As três colunas de ação e a de FII abaixo só são reconhecidas se o rótulo
// acentuado do <thead> ("Mrg. Líq.", "Patrim. Líq", "Dív.Líq/ Patrim.",
// "Vacância Média") chegar íntegro ao índice de colunas. Com mojibake nenhuma
// casa com o mapeamento e a métrica some sem erro.
func TestParseUniverseDecodesLatin1(t *testing.T) {
	stocks := byTicker(universe(t, domain.ClassStock), "WEGE3")
	for _, id := range []domain.MetricID{"mrg_liq", "patrim_liq", "dl_patrim"} {
		require.Contains(t, stocks, id, "coluna de rótulo acentuado não foi reconhecida")
	}

	fiis := byTicker(universe(t, domain.ClassFII), "MXRF11")
	require.Contains(t, fiis, domain.MetricID("vacancia_media"))
}

func TestUniverseParsesStockValues(t *testing.T) {
	wege3 := byTicker(universe(t, domain.ClassStock), "WEGE3")

	want := map[domain.MetricID]float64{
		"cotacao":    49.70,
		"pl":         33.36,
		"pvp":        11.06,
		"dy":         0.0404,
		"roe":        0.3316,
		"patrim_liq": 18_861_400_000,
		"dl_patrim":  -0.20,
	}
	for id, want := range want {
		obs, ok := wege3[id]
		require.True(t, ok, "métrica %q ausente", id)
		require.NotNil(t, obs.Value, "métrica %q veio nula", id)
		require.InDelta(t, want, *obs.Value, 1e-9, "métrica %q", id)
	}
}

func TestUniverseParsesFIIValues(t *testing.T) {
	hglg11 := byTicker(universe(t, domain.ClassFII), "HGLG11")

	want := map[domain.MetricID]float64{
		"cotacao":        147.00,
		"ffo_yield":      0.0662,
		"dy":             0.0731,
		"pvp":            0.89,
		"qtd_imoveis":    60,
		"preco_m2":       1863.47,
		"vacancia_media": 0.0323,
	}
	for id, want := range want {
		obs, ok := hglg11[id]
		require.True(t, ok, "métrica %q ausente", id)
		require.NotNil(t, obs.Value, "métrica %q veio nula", id)
		require.InDelta(t, want, *obs.Value, 1e-9, "métrica %q", id)
	}
}

// O 0,00 de EV/EBITDA que a fonte publica para banco é código de ausência, não
// múltiplo zerado.
func TestUniverseTreatsEvEbitdaZeroAsAbsence(t *testing.T) {
	stocks := universe(t, domain.ClassStock)

	itub4 := byTicker(stocks, "ITUB4")
	obs, ok := itub4["ev_ebitda"]
	require.True(t, ok, "ev_ebitda precisa existir como observação, só que sem valor")
	require.Nil(t, obs.Value)

	// Um zero em coluna sem sentinela continua sendo zero legítimo.
	clan3 := byTicker(stocks, "CLAN3")
	require.NotNil(t, clan3["pl"].Value)
	require.Zero(t, *clan3["pl"].Value)
}

// Nenhuma tabela bulk tem coluna de data: preencher ReferenceAt com FetchedAt
// inventaria uma competência que a fonte nunca informou.
func TestUniverseNeverInventsReferenceAt(t *testing.T) {
	for _, class := range []domain.AssetClass{domain.ClassStock, domain.ClassFII} {
		for _, obs := range universe(t, class) {
			require.Nil(t, obs.ReferenceAt, "%s/%s", obs.Ticker, obs.Metric)
		}
	}
}

func TestUniverseSkipsRowWithoutTickerWithoutAbortingClass(t *testing.T) {
	obs := universe(t, domain.ClassStock)
	require.Len(t, metricIDs(obs), 21)
	require.NotEmpty(t, byTicker(obs, "WEGE3"))
}

// catalog.metrics.yaml e o mapeamento coluna→MetricID do provider foram
// escritos separadamente a partir da mesma tabela de colunas. Sem este teste um
// typo de qualquer um dos lados vira métrica que nunca aparece na tela, e que se
// lê como "nunca coletada" em vez de bug.
func TestCatalogAndProviderMetricIDsMatch(t *testing.T) {
	cat, err := catalog.Load()
	require.NoError(t, err)

	for _, class := range []domain.AssetClass{domain.ClassStock, domain.ClassFII} {
		want := []domain.MetricID{}
		for _, m := range cat.MetricsFor(class) {
			if !m.Derived {
				want = append(want, m.ID)
			}
		}
		require.ElementsMatch(t, want, metricIDs(universe(t, class)),
			"catalog e provider divergem para a classe %s", class)
	}
}

// Complementa o teste acima pelo outro lado: garante que nenhum rótulo do
// <thead> ficou sem mapeamento por descuido. Só três colunas podem sobrar, e
// elas estão nomeadas aqui — "Papel" é a âncora da linha, "Segmento" e
// "Endereço" estão fora do catálogo por decisão de escopo.
func TestEveryHeaderColumnIsMappedOrAllowlisted(t *testing.T) {
	outsideCatalog := map[string]bool{"Papel": true, "Segmento": true, "Endereço": true}

	cases := map[domain.AssetClass]string{
		domain.ClassStock: "resultado_min.html",
		domain.ClassFII:   "fii_resultado_min.html",
	}
	for class, fixture := range cases {
		mappable := 0
		for _, label := range headerLabels(t, fixture) {
			if !outsideCatalog[label] {
				mappable++
			}
		}
		require.Equal(t, mappable, len(metricIDs(universe(t, class))),
			"classe %s: alguma coluna do cabeçalho não vira métrica nem está na allowlist", class)
	}
}

func headerLabels(t *testing.T, fixture string) []string {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", fixture))
	require.NoError(t, err)
	defer f.Close()

	decoded, err := norm.DecodeISO88591(f)
	require.NoError(t, err)
	doc, err := goquery.NewDocumentFromReader(decoded)
	require.NoError(t, err)

	var labels []string
	doc.Find("table.resultado").First().Find("thead th").Each(func(_ int, th *goquery.Selection) {
		labels = append(labels, strings.Join(strings.Fields(th.Text()), " "))
	})
	require.NotEmpty(t, labels)
	return labels
}

func TestSegments(t *testing.T) {
	segments, err := newProvider(t).Segments(t.Context(), false)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"MXRF11": "Logística",
		"ABCP11": "Shoppings",
		"HGLG11": "Multicategoria",
	}, segments)
}
