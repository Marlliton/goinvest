package fundamentus

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/norm"
)

// Uma semana ainda captura um provento novo bem antes da data de pagamento.
const (
	dividendsDocKind = "fundamentus_proventos"
	dividendsTTL     = 7 * 24 * time.Hour
)

const (
	valueLabel   = "Valor"
	typeLabel    = "Tipo"
	paymentLabel = "Data de Pagamento"
	factorLabel  = "Por quantas ações"
)

type dividendSpec struct {
	path      string
	source    string
	dateLabel string
	hasFactor bool
}

func dividendSpecFor(class domain.AssetClass) (dividendSpec, error) {
	switch class {
	case domain.ClassStock:
		return dividendSpec{"/proventos.php", "fundamentus:proventos", "Data", true}, nil
	case domain.ClassFII:
		return dividendSpec{"/fii_proventos.php", "fundamentus:fii_proventos", "Última Data Com", false}, nil
	}
	return dividendSpec{}, fmt.Errorf("fundamentus: unsupported asset class %q", class)
}

func (p *Provider) Dividends(ctx context.Context, ticker string, class domain.AssetClass, force bool) ([]domain.DividendEvent, error) {
	sp, err := dividendSpecFor(class)
	if err != nil {
		return nil, err
	}

	// Os três valores de "tipo" que a fonte aceita devolvem a mesma página; o path decide a classe.
	pageURL := p.baseURL + sp.path + "?papel=" + url.QueryEscape(ticker)
	body, err := p.client.Get(ctx, pageURL, dividendsDocKind, dividendsTTL, force)
	if err != nil {
		return nil, err
	}
	return p.parseDividends(body, ticker, sp)
}

type dividendColumns struct {
	date    int
	value   int
	kind    int
	payment int
	factor  int
}

func (p *Provider) parseDividends(body []byte, ticker string, sp dividendSpec) ([]domain.DividendEvent, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("fundamentus: parse %s: %w", sp.path, err)
	}

	// A página de ação traz uma segunda tabela com o total por ano; o id separa as duas.
	table := doc.Find("table#resultado").First()
	if table.Length() == 0 {
		return nil, fmt.Errorf("fundamentus: %s has no result table", sp.path)
	}

	cols, err := dividendColumnsOf(headerLabels(table), sp)
	if err != nil {
		return nil, err
	}

	at := p.now()
	var out []domain.DividendEvent
	skipped := 0

	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		event, ok := parseDividendRow(row.Find("td"), cols, sp)
		if !ok {
			skipped++
			return
		}
		event.Ticker = ticker
		event.Source = sp.source
		event.FetchedAt = at
		out = append(out, event)
	})

	if len(out) == 0 {
		return nil, fmt.Errorf("fundamentus: %s?papel=%s yielded no usable row (%d discarded)", sp.path, ticker, skipped)
	}
	return out, nil
}

func dividendColumnsOf(labels []string, sp dividendSpec) (dividendColumns, error) {
	cols := dividendColumns{
		date:    indexOf(labels, sp.dateLabel),
		value:   indexOf(labels, valueLabel),
		kind:    indexOf(labels, typeLabel),
		payment: indexOf(labels, paymentLabel),
		factor:  -1,
	}
	required := map[string]int{
		sp.dateLabel: cols.date,
		valueLabel:   cols.value,
		typeLabel:    cols.kind,
		paymentLabel: cols.payment,
	}
	if sp.hasFactor {
		cols.factor = indexOf(labels, factorLabel)
		required[factorLabel] = cols.factor
	}
	for label, at := range required {
		if at < 0 {
			return dividendColumns{}, fmt.Errorf("fundamentus: %s has no %q column", sp.path, label)
		}
	}
	return cols, nil
}

func parseDividendRow(cells *goquery.Selection, cols dividendColumns, sp dividendSpec) (domain.DividendEvent, bool) {
	widest := max(cols.date, cols.value, cols.kind, cols.payment, cols.factor)
	if cells.Length() <= widest {
		return domain.DividendEvent{}, false
	}

	exDate, ok := norm.ParseBRDate(cellText(cells.Eq(cols.date)))
	if !ok {
		return domain.DividendEvent{}, false
	}
	value, ok := norm.ParseBRNumber(cellText(cells.Eq(cols.value)))
	if !ok {
		return domain.DividendEvent{}, false
	}

	// Assumir 1 num evento cotado em lote de mil ações multiplicaria o provento por mil.
	factor := 1.0
	if sp.hasFactor {
		if factor, ok = norm.ParseBRNumber(cellText(cells.Eq(cols.factor))); !ok || factor <= 0 {
			return domain.DividendEvent{}, false
		}
	}

	event := domain.DividendEvent{
		ExDate:           exDate,
		TypeRaw:          cellText(cells.Eq(cols.kind)),
		Type:             classifyDividend(cellText(cells.Eq(cols.kind))),
		ValuePerShareRaw: value,
		SharesFactor:     factor,
	}
	if paid, ok := norm.ParseBRDate(cellText(cells.Eq(cols.payment))); ok {
		event.PaymentDate = &paid
	}
	return event, true
}

// A fonte varia caixa e acento no mesmo campo ao longo dos anos.
func classifyDividend(raw string) domain.DividendType {
	folded := norm.FoldUpper(raw)
	switch {
	case strings.Contains(folded, "DIVIDENDO"):
		return domain.DividendCash
	case strings.Contains(folded, "JUROS"), strings.Contains(folded, "JRS CAP"):
		return domain.DividendJCP
	}
	return domain.DividendUnknown
}
