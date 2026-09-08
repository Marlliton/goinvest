package app

import (
	"context"
	"slices"
	"time"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/store"
)

// As métricas que só a página de detalhe traz. Nenhuma tem equivalente no
// bulk, então "cair para o bulk" aqui significa "não avaliado para todos".
var detailMetrics = []domain.MetricID{
	"lucro_liquido", "ebit", "venda_ativos", "receita", "ffo", "rend_distribuido",
}

const (
	reasonInvalidFormat = "formato inválido para um código da B3"
	reasonNeverSynced   = "não está na base local; rode 'goinvest sync'"
)

type TickerStatus struct {
	Ticker string
	Reason string
}

// Resumo, não a família inteira: comparar oito ativos com cinco dividendos
// anuais cada não cabe, e é para isso que 'goinvest show' serve.
type BazinSummary struct {
	Ceiling             float64
	PremiumDiscount     float64
	NotApplicableReason string
}

type CompareColumn struct {
	Ticker         string
	PeerGroupLabel string
	Values         map[domain.MetricID]*float64
	NotApplicable  map[domain.MetricID]string
	ReferenceAt    map[domain.MetricID]*time.Time
	Bazin          *BazinSummary
	SelicDelta     *float64
	Alerts         []evaluate.Finding
}

type CompareTable struct {
	Class   domain.AssetClass
	Metrics []catalog.Metric
	Columns []CompareColumn
}

type CompareHeader struct {
	SelicRate *float64
	SelicAt   *time.Time
}

type CompareReport struct {
	Header           CompareHeader
	Tables           []CompareTable
	Invalid          []TickerStatus
	DetailMissingFor []domain.MetricID
}

// Compare resolve cada ticker pelo mesmo caminho de Show e decide a
// precedência por (métrica × comparação), nunca por célula: uma coluna com
// metade das linhas vindas do detalhe e metade do bulk misturaria metodologias
// e produziria ranking errado sem nenhum erro visível.
func Compare(ctx context.Context, db *store.DB, cat *catalog.Catalog, tickers []string, now func() time.Time) (CompareReport, error) {
	var report CompareReport

	rate, selicAt, found, err := selic(ctx, db)
	if err != nil {
		return CompareReport{}, err
	}
	if found {
		report.Header = CompareHeader{SelicRate: rate, SelicAt: selicAt}
	}

	loaded := make([]assetData, 0, len(tickers))
	for _, ticker := range tickers {
		if !identity.ValidTicker(ticker) {
			report.Invalid = append(report.Invalid, TickerStatus{ticker, reasonInvalidFormat})
			continue
		}
		data, found, err := loadAsset(ctx, db, ticker)
		if err != nil {
			return CompareReport{}, err
		}
		if !found || len(data.collected) == 0 {
			report.Invalid = append(report.Invalid, TickerStatus{ticker, reasonNeverSynced})
			continue
		}
		loaded = append(loaded, data)
	}

	report.DetailMissingFor = unresolvableDetail(cat, loaded)
	dropped := make(map[domain.MetricID]struct{}, len(report.DetailMissingFor))
	for _, id := range report.DetailMissingFor {
		dropped[id] = struct{}{}
	}

	for _, class := range []domain.AssetClass{domain.ClassStock, domain.ClassFII} {
		table := CompareTable{Class: class}
		for _, data := range loaded {
			if data.asset.Class != class {
				continue
			}
			table.Columns = append(table.Columns, column(cat, data, report.Header, dropped, now()))
		}
		if len(table.Columns) > 0 {
			table.Metrics = cat.MetricsFor(class)
			report.Tables = append(report.Tables, table)
		}
	}
	return report, nil
}

// Um ticker sem a coleta profunda derruba a métrica para todo mundo. Ausência
// estrutural não derruba: ela é uma resposta sobre aquele ativo, e vira não
// aplicável na própria célula.
func unresolvableDetail(cat *catalog.Catalog, loaded []assetData) []domain.MetricID {
	var out []domain.MetricID
	for _, id := range detailMetrics {
		metric, known := cat.Metric(id)
		if !known {
			continue
		}
		for _, data := range loaded {
			if !slices.Contains(metric.Classes, data.asset.Class) {
				continue
			}
			if _, present := data.merged[id]; present {
				continue
			}
			if data.hasDetail && isSentinelSegment(metric, data.asset.Segment) {
				continue
			}
			out = append(out, id)
			break
		}
	}
	return out
}

func column(cat *catalog.Catalog, data assetData, h CompareHeader, dropped map[domain.MetricID]struct{}, now time.Time) CompareColumn {
	asset := data.asset
	view := HeaderView{
		SelicRate: h.SelicRate,
		SelicAt:   h.SelicAt,
		Sector:    asset.Sector,
		Subsector: asset.Subsector,
		Segment:   asset.Segment,
	}

	col := CompareColumn{
		Ticker:        asset.Ticker,
		Values:        map[domain.MetricID]*float64{},
		NotApplicable: map[domain.MetricID]string{},
		ReferenceAt:   map[domain.MetricID]*time.Time{},
		Alerts:        evaluate.Detect(alertInput(cat, asset, data.merged, data.percentiles, view, data.hasDetail)),
	}
	if asset.IsActive {
		col.PeerGroupLabel, _ = peerGroup(asset)
	}

	for _, m := range cat.MetricsFor(asset.Class) {
		if _, out := dropped[m.ID]; out {
			continue
		}
		o, collected := data.merged[m.ID]
		if !collected {
			if data.hasDetail && isSentinelSegment(m, asset.Segment) {
				col.NotApplicable[m.ID] = m.NotApplicable[originSector]
			}
			continue
		}
		if reason := notApplicableReason(m, asset.Segment, data.merged, o.Value); reason != "" {
			col.NotApplicable[m.ID] = reason
			continue
		}
		col.Values[m.ID] = o.Value
		col.ReferenceAt[m.ID] = o.ReferenceAt
	}

	if yield, ok := presentValue(data.merged, dividendYieldID); ok && h.SelicRate != nil {
		delta := yield - *h.SelicRate
		col.SelicDelta = &delta
	}
	col.Bazin = bazinSummary(data, view, now)
	return col
}

func bazinSummary(data assetData, h HeaderView, now time.Time) *BazinSummary {
	v := bazinView(data.events, data.merged, h, data.asset.Ticker, now)
	if v == nil {
		return nil
	}
	return &BazinSummary{
		Ceiling:             v.Ceiling,
		PremiumDiscount:     v.PremiumDiscount,
		NotApplicableReason: v.NotApplicableReason,
	}
}
