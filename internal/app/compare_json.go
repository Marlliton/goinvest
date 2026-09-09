package app

import (
	"encoding/json"
	"time"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
)

const compareSchemaVersion = 2

const (
	statusPresent       = "presente"
	statusNotInformed   = "nao_informado"
	statusNotEvaluated  = "nao_avaliado"
	statusNotApplicable = "nao_aplicavel"
)

type compareDocument struct {
	SchemaVersion    int               `json:"schema_version"`
	Selic            *selicDoc         `json:"selic"`
	Tables           []compareTableDoc `json:"tables"`
	Invalid          []invalidDoc      `json:"invalid"`
	DetailMissingFor []string          `json:"detail_missing_for"`
}

type selicDoc struct {
	Rate        float64 `json:"rate"`
	ReferenceAt *string `json:"reference_at"`
}

type compareTableDoc struct {
	Class   string             `json:"class"`
	Columns []compareColumnDoc `json:"columns"`
}

type compareColumnDoc struct {
	Ticker     string               `json:"ticker"`
	PeerGroup  string               `json:"peer_group"`
	SelicDelta *float64             `json:"selic_delta"`
	Metrics    map[string]metricDoc `json:"metrics"`
	Bazin      *bazinDoc            `json:"bazin"`
	Alerts     []alertDoc           `json:"alerts"`
}

type metricDoc struct {
	Value               *float64 `json:"value"`
	Unit                string   `json:"unit"`
	Status              string   `json:"status"`
	Source              string   `json:"source"`
	ReferenceAt         *string  `json:"reference_at"`
	FetchedAt           *string  `json:"fetched_at"`
	NotApplicableReason string   `json:"not_applicable_reason"`
}

type bazinDoc struct {
	Ceiling             float64    `json:"ceiling"`
	CurrentPrice        float64    `json:"current_price"`
	PremiumDiscount     float64    `json:"premium_discount"`
	YearsUsed           int        `json:"years_used"`
	Years               []yearDoc  `json:"years"`
	AtypicalYears       []int      `json:"atypical_years"`
	Gordon              *gordonDoc `json:"gordon"`
	MedianDividendYield *float64   `json:"median_dividend_yield"`
	TwelveMonthYield    *float64   `json:"twelve_month_yield"`
	NotApplicableReason string     `json:"not_applicable_reason"`
}

type gordonDoc struct {
	Ceiling             float64 `json:"ceiling"`
	RequiredReturn      float64 `json:"required_return"`
	ImpliedGrowth       float64 `json:"implied_growth"`
	NotApplicableReason string  `json:"not_applicable_reason"`
}

type yearDoc struct {
	Year  int     `json:"year"`
	Total float64 `json:"total"`
}

type alertDoc struct {
	ID      string             `json:"id"`
	Status  string             `json:"status"`
	Rule    string             `json:"rule"`
	Numbers map[string]float64 `json:"numbers"`
	Reason  string             `json:"reason"`
}

type invalidDoc struct {
	Ticker string `json:"ticker"`
	Reason string `json:"reason"`
}

func RenderCompareJSON(r CompareReport) ([]byte, error) {
	doc := compareDocument{
		SchemaVersion:    compareSchemaVersion,
		Tables:           []compareTableDoc{},
		Invalid:          []invalidDoc{},
		DetailMissingFor: []string{},
	}

	if r.Header.SelicRate != nil {
		doc.Selic = &selicDoc{Rate: *r.Header.SelicRate, ReferenceAt: rfc3339(r.Header.SelicAt)}
	}
	for _, id := range r.DetailMissingFor {
		doc.DetailMissingFor = append(doc.DetailMissingFor, string(id))
	}
	for _, s := range r.Invalid {
		doc.Invalid = append(doc.Invalid, invalidDoc{Ticker: s.Ticker, Reason: s.Reason})
	}

	missing := make(map[domain.MetricID]struct{}, len(r.DetailMissingFor))
	for _, id := range r.DetailMissingFor {
		missing[id] = struct{}{}
	}

	for _, table := range r.Tables {
		t := compareTableDoc{Class: string(table.Class), Columns: []compareColumnDoc{}}
		for _, col := range table.Columns {
			t.Columns = append(t.Columns, columnDoc(col, table.Metrics, missing))
		}
		doc.Tables = append(doc.Tables, t)
	}

	return json.MarshalIndent(doc, "", "  ")
}

func columnDoc(col CompareColumn, metrics []catalog.Metric, missing map[domain.MetricID]struct{}) compareColumnDoc {
	out := compareColumnDoc{
		Ticker:     col.Ticker,
		PeerGroup:  col.PeerGroupLabel,
		SelicDelta: col.SelicDelta,
		Metrics:    make(map[string]metricDoc, len(metrics)),
		Bazin:      bazinDocOf(col.Bazin),
		Alerts:     make([]alertDoc, 0, len(col.Alerts)),
	}

	for _, m := range metrics {
		out.Metrics[string(m.ID)] = cellDoc(col.Cells[m.ID], m, hasKey(col.Cells, m.ID), missing)
	}
	for _, f := range col.Alerts {
		out.Alerts = append(out.Alerts, alertDoc{
			ID: f.ID, Status: string(f.Status), Rule: f.Rule,
			Numbers: f.Numbers, Reason: f.Reason,
		})
	}
	return out
}

func cellDoc(cell MetricCell, m catalog.Metric, collected bool, missing map[domain.MetricID]struct{}) metricDoc {
	doc := metricDoc{
		Unit:                string(m.Unit),
		Source:              cell.Source,
		ReferenceAt:         rfc3339(cell.ReferenceAt),
		NotApplicableReason: cell.NotApplicableReason,
	}
	if !cell.FetchedAt.IsZero() {
		doc.FetchedAt = rfc3339(&cell.FetchedAt)
	}

	switch {
	case cell.NotApplicableReason != "":
		doc.Status = statusNotApplicable
	case !collected:
		doc.Status = statusNotEvaluated
	case cell.Value == nil:
		doc.Status = statusNotInformed
	default:
		doc.Status = statusPresent
		doc.Value = cell.Value
	}
	return doc
}

func bazinDocOf(v *BazinView) *bazinDoc {
	if v == nil {
		return nil
	}
	doc := &bazinDoc{
		Ceiling:             v.Ceiling,
		CurrentPrice:        v.CurrentPrice,
		PremiumDiscount:     v.PremiumDiscount,
		YearsUsed:           v.YearsUsed,
		Years:               make([]yearDoc, 0, len(v.Years)),
		AtypicalYears:       v.AtypicalYears,
		Gordon:              gordonDocOf(v.Gordon),
		MedianDividendYield: v.MedianDividendYield,
		TwelveMonthYield:    v.TwelveMonthYield,
		NotApplicableReason: v.NotApplicableReason,
	}
	if doc.AtypicalYears == nil {
		doc.AtypicalYears = []int{}
	}
	for _, y := range v.Years {
		doc.Years = append(doc.Years, yearDoc{Year: y.Year, Total: y.Total})
	}
	return doc
}

func gordonDocOf(v *GordonView) *gordonDoc {
	if v == nil {
		return nil
	}
	return &gordonDoc{
		Ceiling:             v.Ceiling,
		RequiredReturn:      v.RequiredReturn,
		ImpliedGrowth:       v.ImpliedGrowth,
		NotApplicableReason: v.NotApplicableReason,
	}
}

func hasKey(cells map[domain.MetricID]MetricCell, id domain.MetricID) bool {
	_, ok := cells[id]
	return ok
}

func rfc3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}
