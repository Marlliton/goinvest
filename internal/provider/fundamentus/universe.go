// Package fundamentus lê as duas tabelas bulk do Fundamentus: uma requisição
// por classe devolve o mercado inteiro.
package fundamentus

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/norm"
)

// Fundamentalista muda por trimestre; meio dia de cache ainda deixa o usuário
// sincronizar de manhã e de tarde sem bater duas vezes na fonte.
const universeTTL = 12 * time.Hour

// A coluna do ticker é a âncora da linha: sem ela não há o que identificar.
const tickerLabel = "Papel"

type parser func(string) (float64, bool)

type column struct {
	metric domain.MetricID
	unit   domain.Unit
	parse  parser
}

func number(metric domain.MetricID, unit domain.Unit) column {
	return column{metric: metric, unit: unit, parse: norm.ParseBRNumber}
}

func percent(metric domain.MetricID) column {
	return column{metric: metric, unit: domain.UnitPercent, parse: norm.ParseBRPercent}
}

// As chaves são os rótulos do <thead>, exatamente como a fonte os escreve.
var stockColumns = map[string]column{
	"Cotação":          number("cotacao", domain.UnitBRL),
	"P/L":              number("pl", domain.UnitRatio),
	"P/VP":             number("pvp", domain.UnitRatio),
	"PSR":              number("psr", domain.UnitRatio),
	"Div.Yield":        percent("dy"),
	"P/Ativo":          number("p_ativo", domain.UnitRatio),
	"P/Cap.Giro":       number("p_cap_giro", domain.UnitRatio),
	"P/EBIT":           number("p_ebit", domain.UnitRatio),
	"P/Ativ Circ.Liq":  number("p_ativ_circ_liq", domain.UnitRatio),
	"EV/EBIT":          number("ev_ebit", domain.UnitRatio),
	"EV/EBITDA":        number("ev_ebitda", domain.UnitRatio),
	"Mrg Bruta":        percent("mrg_bruta"),
	"Mrg Ebit":         percent("mrg_ebit"),
	"Mrg. Líq.":        percent("mrg_liq"),
	"Liq. Corr.":       number("liq_corr", domain.UnitRatio),
	"ROIC":             percent("roic"),
	"ROE":              percent("roe"),
	"Liq.2meses":       number("liq_2meses", domain.UnitBRL),
	"Patrim. Líq":      number("patrim_liq", domain.UnitBRL),
	"Dív.Líq/ Patrim.": number("dl_patrim", domain.UnitRatio),
	"Cresc. Rec.5a":    percent("cresc_rec_5a"),
}

// Segmento e Endereço ficam de fora por decisão de escopo do catálogo, não por
// esquecimento.
var fiiColumns = map[string]column{
	"Cotação":          number("cotacao", domain.UnitBRL),
	"FFO Yield":        percent("ffo_yield"),
	"Dividend Yield":   percent("dy"),
	"P/VP":             number("pvp", domain.UnitRatio),
	"Valor de Mercado": number("valor_mercado", domain.UnitBRL),
	"Liquidez":         number("liquidez_fii", domain.UnitBRL),
	"Qtd de imóveis":   number("qtd_imoveis", domain.UnitCount),
	"Preço do m2":      number("preco_m2", domain.UnitBRL),
	"Aluguel por m2":   number("aluguel_m2", domain.UnitBRL),
	"Cap Rate":         percent("cap_rate"),
	"Vacância Média":   percent("vacancia_media"),
}

type spec struct {
	path    string
	source  string
	columns map[string]column
}

func specFor(class domain.AssetClass) (spec, error) {
	switch class {
	case domain.ClassStock:
		return spec{"/resultado.php", "fundamentus:resultado", stockColumns}, nil
	case domain.ClassFII:
		return spec{"/fii_resultado.php", "fundamentus:fii_resultado", fiiColumns}, nil
	}
	return spec{}, fmt.Errorf("fundamentus: unsupported asset class %q", class)
}

type Provider struct {
	client  *fetch.Client
	baseURL string
	now     func() time.Time
}

func NewProvider(client *fetch.Client, baseURL string, now func() time.Time) *Provider {
	if now == nil {
		now = time.Now
	}
	return &Provider{client: client, baseURL: strings.TrimSuffix(baseURL, "/"), now: now}
}

func (p *Provider) Name() string { return "fundamentus" }

func (p *Provider) SourceID(class domain.AssetClass) string {
	sp, err := specFor(class)
	if err != nil {
		return p.Name()
	}
	return sp.source
}

func (p *Provider) Universe(ctx context.Context, class domain.AssetClass, force bool) ([]domain.Observation, error) {
	sp, err := specFor(class)
	if err != nil {
		return nil, err
	}

	body, err := p.client.Get(ctx, p.baseURL+sp.path, "universe_"+string(class), universeTTL, force)
	if err != nil {
		return nil, err
	}
	return p.parse(body, sp)
}

const segmentLabel = "Segmento"

// Segments lê a mesma página que Universe já baixa para FIIs: dentro do TTL a
// chamada não custa requisição nenhuma. O segmento não é métrica, é o setor
// provisório do fundo até a fonte definitiva existir.
func (p *Provider) Segments(ctx context.Context, force bool) (map[string]string, error) {
	sp, err := specFor(domain.ClassFII)
	if err != nil {
		return nil, err
	}

	body, err := p.client.Get(ctx, p.baseURL+sp.path, "universe_"+string(domain.ClassFII), universeTTL, force)
	if err != nil {
		return nil, err
	}

	table, labels, err := resultTable(body, sp)
	if err != nil {
		return nil, err
	}
	tickerAt := indexOf(labels, tickerLabel)
	if tickerAt < 0 {
		return nil, fmt.Errorf("fundamentus: %s has no %q column", sp.path, tickerLabel)
	}
	segmentAt := indexOf(labels, segmentLabel)
	if segmentAt < 0 {
		return nil, fmt.Errorf("fundamentus: %s has no %q column", sp.path, segmentLabel)
	}

	out := make(map[string]string)
	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() <= tickerAt || cells.Length() <= segmentAt {
			return
		}
		ticker := cellText(cells.Eq(tickerAt))
		segment := cellText(cells.Eq(segmentAt))
		if ticker == "" || segment == "" {
			return
		}
		out[ticker] = segment
	})
	return out, nil
}

func resultTable(body []byte, sp spec) (*goquery.Selection, []string, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, nil, fmt.Errorf("fundamentus: parse %s: %w", sp.path, err)
	}

	table := doc.Find("table.resultado").First()
	if table.Length() == 0 {
		return nil, nil, fmt.Errorf("fundamentus: %s has no result table", sp.path)
	}
	return table, headerLabels(table), nil
}

func (p *Provider) parse(body []byte, sp spec) ([]domain.Observation, error) {
	table, labels, err := resultTable(body, sp)
	if err != nil {
		return nil, err
	}

	tickerAt := indexOf(labels, tickerLabel)
	if tickerAt < 0 {
		return nil, fmt.Errorf("fundamentus: %s has no %q column", sp.path, tickerLabel)
	}

	at := p.now()
	var out []domain.Observation
	skipped := 0

	table.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		cells := row.Find("td")
		if cells.Length() <= tickerAt {
			skipped++
			return
		}
		ticker := cellText(cells.Eq(tickerAt))
		if ticker == "" {
			skipped++
			return
		}

		cells.Each(func(i int, cell *goquery.Selection) {
			if i >= len(labels) {
				return
			}
			col, ok := sp.columns[labels[i]]
			if !ok {
				return
			}
			out = append(out, domain.Observation{
				Ticker:     ticker,
				Metric:     col.metric,
				PeriodKind: "spot",
				PeriodEnd:  at,
				Value:      col.value(cellText(cell)),
				Unit:       col.unit,
				Source:     sp.source,
				FetchedAt:  at,
			})
		})
	})

	// Uma linha quebrada é ruído da fonte; nenhuma linha aproveitável é a
	// página tendo mudado de forma.
	if len(out) == 0 {
		return nil, fmt.Errorf("fundamentus: %s yielded no usable row (%d discarded without %q)", sp.path, skipped, tickerLabel)
	}
	return out, nil
}

// value devolve nil tanto para ausência declarada pela fonte quanto para o
// valor numérico que aquela métrica usa como código de ausência.
func (c column) value(text string) *float64 {
	v, ok := c.parse(text)
	if !ok || norm.IsAbsenceSentinel(c.metric, v) {
		return nil
	}
	return &v
}

// O índice de colunas vem do <thead> a cada coleta: posição fixa quebraria em
// silêncio no dia em que a fonte inserir uma coluna.
func headerLabels(table *goquery.Selection) []string {
	var labels []string
	table.Find("thead th").Each(func(_ int, th *goquery.Selection) {
		labels = append(labels, cellText(th))
	})
	return labels
}

func cellText(s *goquery.Selection) string {
	return strings.Join(strings.Fields(s.Text()), " ")
}

func indexOf(labels []string, want string) int {
	for i, l := range labels {
		if l == want {
			return i
		}
	}
	return -1
}
