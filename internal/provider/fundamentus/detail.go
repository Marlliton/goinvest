package fundamentus

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/norm"
)

const (
	detailPath    = "/detalhes.php"
	detailSource  = "fundamentus:detalhes"
	detailDocKind = "fundamentus_detalhes"
	detailTTL     = 24 * time.Hour
)

// "Receita" só existe na página de FII; a de ação traz "Receita Líquida", que é
// outra métrica.
var detailColumns = map[string]domain.MetricID{
	"Lucro Líquido":     "lucro_liquido",
	"EBIT":              "ebit",
	"Venda de ativos":   "venda_ativos",
	"FFO":               "ffo",
	"Rend. Distribuído": "rend_distribuido",
	"Receita":           "receita",
}

func detailTickerLabel(class domain.AssetClass) (string, error) {
	switch class {
	case domain.ClassStock:
		return "Papel", nil
	case domain.ClassFII:
		return "FII", nil
	}
	return "", fmt.Errorf("fundamentus: unsupported asset class %q", class)
}

// Rótulo ausente não vira chave no MetricSet: separar "a fonte não publica" de
// "não avaliamos" depende do documento em cache, e essa leitura é da aplicação.
func (p *Provider) Detail(ctx context.Context, ticker string, class domain.AssetClass, force bool) (domain.MetricSet, error) {
	tickerLabel, err := detailTickerLabel(class)
	if err != nil {
		return nil, err
	}

	pageURL := p.baseURL + detailPath + "?papel=" + url.QueryEscape(ticker)
	body, err := p.client.Get(ctx, pageURL, detailDocKind, detailTTL, force)
	if err != nil {
		return nil, err
	}
	return p.parseDetail(body, ticker, tickerLabel)
}

func (p *Provider) parseDetail(body []byte, ticker, tickerLabel string) (domain.MetricSet, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fundamentus: parse %s: %w", detailPath, err)
	}

	cells := doc.Find("td")
	texts := make([]string, cells.Length())
	cells.Each(func(i int, cell *goquery.Selection) { texts[i] = spanText(cell) })

	at := p.now()
	set := domain.MetricSet{}
	tickerFound := false

	for i := 0; i+1 < len(texts); i++ {
		if texts[i] == tickerLabel && texts[i+1] == ticker {
			tickerFound = true
			continue
		}
		metric, ok := detailColumns[texts[i]]
		if !ok {
			continue
		}
		// O mesmo rótulo se repete adiante para a coluna de 3 meses; a
		// primeira ocorrência é a de 12 meses.
		if _, dup := set[metric]; dup {
			continue
		}
		value, ok := norm.ParseBRNumber(texts[i+1])
		if !ok {
			continue
		}
		set[metric] = domain.Observation{
			Ticker:     ticker,
			Metric:     metric,
			PeriodKind: "ttm",
			PeriodEnd:  at,
			Value:      &value,
			Unit:       domain.UnitBRL,
			Source:     detailSource,
			FetchedAt:  at,
		}
	}

	if len(set) == 0 && !tickerFound {
		return nil, fmt.Errorf("fundamentus: %s?papel=%s has neither %q nor any known metric label", detailPath, ticker, tickerLabel)
	}
	return set, nil
}

// cell.Text() traria junto o "?" da ajuda que abre a célula.
func spanText(cell *goquery.Selection) string {
	return cellText(cell.Find("span.txt").First())
}
