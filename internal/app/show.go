package app

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/bazin"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/config"
	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/evaluate"
	"github.com/marlliton/goinvest/internal/store"
)

var ErrNoData = errors.New("nenhum dado local. Rode 'goinvest sync' primeiro")

const stalenessThreshold = 7 * 24 * time.Hour

const (
	dividendYieldID = domain.MetricID("dy")
	priceID         = domain.MetricID("cotacao")
)

type HeaderView struct {
	ReferenceAt        *time.Time
	FetchedAt          time.Time
	Age                time.Duration
	Stale              bool
	Inactive           bool
	LastLiquidAt       *time.Time
	Sector             string
	Subsector          string
	Segment            string
	IncompleteRegistry int
	TotalInClass       int
	PeerGroupLabel     string
	PeerGroupN         int
	SelicRate          *float64
	SelicAt            *time.Time
}

type LineView struct {
	MetricID            domain.MetricID
	Label               string
	Value               *float64
	Unit                domain.Unit
	Derived             bool
	Formula             string
	Percentile          *float64
	PeerN               *int
	FellBackToMarket    bool
	SelicDelta          *float64
	ReferenceAt         *time.Time
	AlertMarks          []string
	NotApplicableReason string
}

type BlockView struct {
	Label string
	Lines []LineView
}

type SensitivityPoint struct {
	Label   string
	Growth  float64
	Ceiling float64
}

type GordonView struct {
	Ceiling             float64
	RequiredReturn      float64
	ImpliedGrowth       float64
	Sensitivity         []SensitivityPoint
	NotApplicableReason string
}

type BazinView struct {
	Ceiling             float64
	CurrentPrice        float64
	PremiumDiscount     float64
	YearsUsed           int
	Years               []bazin.YearlyDividend
	AtypicalYears       []int
	Gordon              *GordonView
	MedianDividendYield *float64
	TwelveMonthYield    *float64
	NotApplicableReason string
}

type Report struct {
	Ticker          string
	Class           domain.AssetClass
	Header          HeaderView
	Blocks          []BlockView
	Bazin           *BazinView
	ExpectedReturn  *ExpectedReturnView
	OpportunityCost *OpportunityCostView
	Alerts          []evaluate.Finding
}

func Show(ctx context.Context, db *store.DB, cat *catalog.Catalog, ticker string, tax config.Result, now func() time.Time) (Report, error) {
	data, found, err := loadAsset(ctx, db, ticker)
	if err != nil {
		return Report{}, err
	}
	if !found || len(data.collected) == 0 {
		return Report{}, ErrNoData
	}
	asset := data.asset

	total, withSector, err := db.SectorCoverage(ctx, asset.Class)
	if err != nil {
		return Report{}, err
	}

	h := header(data.collected, now)
	h.Inactive = !asset.IsActive
	h.LastLiquidAt = asset.LastLiquidAt
	h.Sector, h.Subsector, h.Segment = asset.Sector, asset.Subsector, asset.Segment
	h.TotalInClass = total
	h.IncompleteRegistry = total - withSector

	rate, selicAt, found, err := selic(ctx, db)
	if err != nil {
		return Report{}, err
	}
	if found {
		h.SelicRate, h.SelicAt = rate, selicAt
	}
	if asset.IsActive {
		h.PeerGroupLabel, h.PeerGroupN = peerGroup(asset)
	}

	alerts := evaluate.Detect(alertInput(cat, asset, data.merged, data.percentiles, h, data.hasDetail, data.events, now()))

	expectedReturn := expectedReturnView(asset.Class, data.merged)

	cdiRate, err := cdi(ctx, db)
	if err != nil {
		return Report{}, err
	}
	ipca10yRate, err := tesouroIPCA10y(ctx, db)
	if err != nil {
		return Report{}, err
	}
	focusRate, err := focusIPCA12m(ctx, db)
	if err != nil {
		return Report{}, err
	}

	var distributed, growth *float64
	if asset.Class == domain.ClassFII {
		if dy, ok := presentValue(data.merged, dividendYieldID); ok {
			distributed = &dy
		}
	} else if expectedReturn.NotApplicableReason == "" {
		d, g := expectedReturn.Distributed, expectedReturn.ImpliedGrowth
		distributed, growth = &d, &g
	}

	opportunityCost := opportunityCostView(
		macroRate{Rate: h.SelicRate, ReferenceAt: h.SelicAt},
		cdiRate, ipca10yRate, focusRate, distributed, growth, tax)

	return Report{
		Ticker:          asset.Ticker,
		Class:           asset.Class,
		Header:          h,
		Blocks:          blocks(cat, asset, data.merged, data.percentiles, h, data.hasDetail, alerts),
		Bazin:           bazinView(data.events, data.merged, h, asset.Ticker, asset.Class, now()),
		ExpectedReturn:  expectedReturn,
		OpportunityCost: &opportunityCost,
		Alerts:          alerts,
	}, nil
}

type assetData struct {
	asset       domain.Asset
	collected   domain.MetricSet
	merged      domain.MetricSet
	hasDetail   bool
	percentiles map[domain.MetricID]store.AssetPercentile
	events      []domain.DividendEvent
}

func loadAsset(ctx context.Context, db *store.DB, ticker string) (assetData, bool, error) {
	asset, found, err := db.GetAsset(ctx, ticker)
	if err != nil || !found {
		return assetData{}, false, err
	}

	collected, err := db.LatestMetrics(ctx, asset.AssetID, asset.Ticker)
	if err != nil {
		return assetData{}, false, err
	}

	merged := maps.Clone(collected)
	for id, o := range derive.Compute(collected) {
		if _, taken := merged[id]; !taken {
			merged[id] = o
		}
	}

	hasDetail, err := db.HasDetail(ctx, asset.AssetID)
	if err != nil {
		return assetData{}, false, err
	}

	var percentiles map[domain.MetricID]store.AssetPercentile
	if asset.IsActive {
		percentiles, err = assetPercentiles(ctx, db, asset.AssetID)
		if err != nil {
			return assetData{}, false, err
		}
	}

	events, err := db.ListDividendEvents(ctx, asset.AssetID)
	if err != nil {
		return assetData{}, false, err
	}

	return assetData{
		asset:       asset,
		collected:   collected,
		merged:      merged,
		hasDetail:   hasDetail,
		percentiles: percentiles,
		events:      events,
	}, true, nil
}

func selic(ctx context.Context, db *store.DB) (rate *float64, at *time.Time, found bool, err error) {
	rate, at, _, found, err = db.GetSelic(ctx)
	return rate, at, found, err
}

func cdi(ctx context.Context, db *store.DB) (macroRate, error) {
	rate, at, _, found, err := db.GetCDI(ctx)
	if err != nil || !found {
		return macroRate{}, err
	}
	return macroRate{Rate: rate, ReferenceAt: at}, nil
}

func tesouroIPCA10y(ctx context.Context, db *store.DB) (macroRate, error) {
	rate, at, _, found, err := db.GetTesouroIPCA10y(ctx)
	if err != nil || !found {
		return macroRate{}, err
	}
	return macroRate{Rate: rate, ReferenceAt: at}, nil
}

func focusIPCA12m(ctx context.Context, db *store.DB) (macroRate, error) {
	rate, at, _, found, err := db.GetFocusIPCA12m(ctx)
	if err != nil || !found {
		return macroRate{}, err
	}
	return macroRate{Rate: rate, ReferenceAt: at}, nil
}

func alertInput(cat *catalog.Catalog, asset domain.Asset, merged domain.MetricSet, percentiles map[domain.MetricID]store.AssetPercentile, h HeaderView, hasDetail bool, events []domain.DividendEvent, now time.Time) evaluate.Input {
	in := evaluate.Input{
		Class:     asset.Class,
		Metrics:   merged,
		Segment:   taxonomyLabel(asset),
		HasDetail: hasDetail,
		SelicRate: h.SelicRate,
	}
	if p, ok := percentiles[dividendYieldID]; ok {
		in.DYPercentile = &p.Percentile
	}
	if ebit, ok := cat.Metric(ebitID); ok {
		in.EBITNotApplicableReason = ebit.NotApplicable[originSector]
	}
	if share, ok := bazin.CurrentYearConcentration(events, now); ok {
		in.CurrentYearConcentration = &share
	}
	return in
}

// FII tem taxonomia de um nível: o rótulo chega em Sector, e ler Segment
// deixaria o alerta de vacância sempre não aplicável.
func taxonomyLabel(asset domain.Asset) string {
	if asset.Class == domain.ClassFII {
		return asset.Sector
	}
	return asset.Segment
}

var alertAnchors = map[string]domain.MetricID{
	"ALERTA-01": "payout",
	"ALERTA-02": "pvp",
	"ALERTA-03": "lucro_liquido",
	"ALERTA-04": "rend_distribuido",
	"ALERTA-05": "vacancia_media",
	"ALERTA-08": dividendYieldID,
	"ALERTA-09": domain.MetricID("pl"),
}

func marksFor(alerts []evaluate.Finding, id domain.MetricID) []string {
	var out []string
	for _, f := range alerts {
		if f.Status == evaluate.StatusFired && alertAnchors[f.ID] == id {
			out = append(out, f.ID)
		}
	}
	return out
}

func bazinView(events []domain.DividendEvent, merged domain.MetricSet, h HeaderView, ticker string, class domain.AssetClass, now time.Time) *BazinView {
	if len(events) == 0 {
		return &BazinView{NotApplicableReason: "nenhum provento coletado; rode 'goinvest detalhar " + ticker + "'"}
	}

	result, ok := bazin.Compute(events, now)
	if !ok {
		return &BazinView{NotApplicableReason: ceilingRefusal(result)}
	}

	price, hasPrice := presentValue(merged, priceID)
	if !hasPrice {
		return &BazinView{NotApplicableReason: "cotação não coletada"}
	}
	// O teto sem o DY−Selic ao lado se lê como veredito: a renda fixa é a âncora.
	if h.SelicRate == nil {
		return &BazinView{NotApplicableReason: "Selic desconhecida; rode 'goinvest sync'"}
	}
	if _, hasYield := presentValue(merged, dividendYieldID); !hasYield {
		return &BazinView{NotApplicableReason: "DY não coletado; o teto não sai sem o DY−Selic ao lado"}
	}
	if result.Ceiling <= 0 {
		return &BazinView{NotApplicableReason: "os proventos da janela somam zero"}
	}

	v := &BazinView{
		Ceiling:         result.Ceiling,
		CurrentPrice:    price,
		PremiumDiscount: (price - result.Ceiling) / result.Ceiling,
		YearsUsed:       result.YearsAvailable,
		Years:           result.Years,
		AtypicalYears:   result.AtypicalYears,
	}
	if median, ok := bazin.MedianDividendYield(result, price); ok {
		v.MedianDividendYield = &median
	}
	if dy, hasYield := presentValue(merged, dividendYieldID); hasYield {
		v.TwelveMonthYield = &dy
	}
	v.Gordon = gordonView(class, merged, price, h.SelicRate)
	return v
}

const growthID5y = domain.MetricID("cresc_rec_5a")

func gordonView(class domain.AssetClass, merged domain.MetricSet, price float64, selicRate *float64) *GordonView {
	if class == domain.ClassFII {
		return &GordonView{NotApplicableReason: "Gordon depende de ROE e retenção, que não existem para FII"}
	}

	dec, ok := derive.Decompose(merged)
	if !ok {
		return &GordonView{NotApplicableReason: "crescimento implícito não calculável: P/L, DY, ROE ou payout ausente ou empresa com prejuízo"}
	}
	if selicRate == nil {
		return &GordonView{
			ImpliedGrowth:       dec.ImpliedGrowth,
			NotApplicableReason: "Selic desconhecida; o retorno exigido dependeria dela",
		}
	}

	required := bazin.RequiredReturn(*selicRate)
	dividendPerShare := dec.Distributed * price
	gr, ok := bazin.Gordon(dividendPerShare, required, dec.ImpliedGrowth)
	if !ok {
		return &GordonView{
			RequiredReturn:      required,
			ImpliedGrowth:       dec.ImpliedGrowth,
			NotApplicableReason: "crescimento implícito maior ou igual ao retorno exigido: a fórmula de Gordon estoura",
		}
	}

	sensitivity := []SensitivityPoint{{Label: "g implícito", Growth: dec.ImpliedGrowth, Ceiling: gr.Ceiling}}
	if cresc5a, has5a := presentValue(merged, growthID5y); has5a && cresc5a < required {
		if ceiling5a, ok := bazin.Gordon(dividendPerShare, required, cresc5a); ok {
			sensitivity = append(sensitivity, SensitivityPoint{Label: "histórico 5a", Growth: cresc5a, Ceiling: ceiling5a.Ceiling})
		}
	}
	if ceilingZero, ok := bazin.Gordon(dividendPerShare, required, 0); ok {
		sensitivity = append(sensitivity, SensitivityPoint{Label: "crescimento zero", Growth: 0, Ceiling: ceilingZero.Ceiling})
	}

	return &GordonView{
		Ceiling:        gr.Ceiling,
		RequiredReturn: required,
		ImpliedGrowth:  dec.ImpliedGrowth,
		Sensitivity:    sensitivity,
	}
}

func ceilingRefusal(result bazin.Result) string {
	if len(result.MissingYears) > 0 {
		years := make([]string, 0, len(result.MissingYears))
		for _, y := range result.MissingYears {
			years = append(years, strconv.Itoa(y))
		}
		return "sem provento em " + strings.Join(years, ", ") +
			"; o método não vale para quem não paga com consistência"
	}
	return fmt.Sprintf("só %s com provento; o método pede ao menos %d",
		plural(result.YearsAvailable, "exercício fechado", "exercícios fechados"), bazin.MinYears)
}

func presentValue(merged domain.MetricSet, id domain.MetricID) (float64, bool) {
	o, ok := merged[id]
	if !ok || o.Value == nil {
		return 0, false
	}
	return *o.Value, true
}

func peerGroup(asset domain.Asset) (label string, n int) {
	if asset.PeerGroupLevel == "" {
		return "", 0
	}
	if asset.PeerGroupN != nil {
		n = *asset.PeerGroupN
	}
	if asset.PeerGroupLevel == "mercado" {
		return "o mercado", n
	}
	return asset.PeerGroupKey, n
}

func assetPercentiles(ctx context.Context, db *store.DB, assetID int64) (map[domain.MetricID]store.AssetPercentile, error) {
	rows, err := db.GetAssetPercentiles(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.MetricID]store.AssetPercentile, len(rows))
	for _, r := range rows {
		out[r.MetricID] = r
	}
	return out, nil
}

func header(collected domain.MetricSet, now func() time.Time) HeaderView {
	h := HeaderView{ReferenceAt: commonReference(collected)}
	for _, id := range sortedIDs(collected) {
		if o := collected[id]; o.FetchedAt.After(h.FetchedAt) {
			h.FetchedAt = o.FetchedAt
		}
	}
	h.Age = now().Sub(h.FetchedAt)
	h.Stale = h.Age > stalenessThreshold
	return h
}

func commonReference(collected domain.MetricSet) *time.Time {
	count := map[int64]int{}
	for _, o := range collected {
		count[referenceKey(o.ReferenceAt)]++
	}

	var best int64
	bestCount := -1
	for _, id := range sortedIDs(collected) {
		k := referenceKey(collected[id].ReferenceAt)
		if count[k] > bestCount {
			best, bestCount = k, count[k]
		}
	}

	if best == referenceKeyAbsent {
		return nil
	}
	t := time.Unix(best, 0).UTC()
	return &t
}

const referenceKeyAbsent = int64(-1)

func referenceKey(t *time.Time) int64 {
	if t == nil {
		return referenceKeyAbsent
	}
	return t.UTC().Unix()
}

func blocks(cat *catalog.Catalog, asset domain.Asset, merged domain.MetricSet, percentiles map[domain.MetricID]store.AssetPercentile, h HeaderView, hasDetail bool, alerts []evaluate.Finding) []BlockView {
	applicable := cat.MetricsFor(asset.Class)

	out := make([]BlockView, 0, len(cat.Blocks))
	for _, b := range cat.BlocksOrdered() {
		view := BlockView{Label: b.Label}
		for _, m := range applicable {
			if m.Block != b.ID {
				continue
			}
			o, ok := merged[m.ID]
			if !ok {
				if hasDetail && isSentinelSegment(m, asset.Segment) {
					view.Lines = append(view.Lines, notApplicableLine(m, m.NotApplicable[originSector]))
				}
				continue
			}
			if reason := notApplicableReason(m, asset.Segment, merged, o.Value); reason != "" {
				view.Lines = append(view.Lines, notApplicableLine(m, reason))
				continue
			}
			line := LineView{
				MetricID:    m.ID,
				Label:       m.Label,
				Value:       o.Value,
				Unit:        m.Unit,
				Derived:     m.Derived,
				Formula:     m.Formula,
				ReferenceAt: o.ReferenceAt,
				AlertMarks:  marksFor(alerts, m.ID),
			}
			if m.ID == dividendYieldID && h.SelicRate != nil && o.Value != nil {
				delta := *o.Value - *h.SelicRate
				line.SelicDelta = &delta
			}
			if p, ok := percentiles[m.ID]; ok && m.Percentile {
				n := p.N
				line.Percentile = &p.Percentile
				line.PeerN = &n
				line.FellBackToMarket = p.FellBackToMarket
			}
			view.Lines = append(view.Lines, line)
		}
		if len(view.Lines) > 0 {
			out = append(out, view)
		}
	}
	return out
}

const (
	originSector = "setor"
	originAsset  = "ativo"
)

const (
	equityID = domain.MetricID("patrim_liq")
	ebitID   = domain.MetricID("ebit")
)

func notApplicableLine(m catalog.Metric, reason string) LineView {
	return LineView{
		MetricID:            m.ID,
		Label:               m.Label,
		Unit:                m.Unit,
		Derived:             m.Derived,
		Formula:             m.Formula,
		NotApplicableReason: reason,
	}
}

// Patrimônio não positivo precede a sentinela: o múltiplo é calculável e mentiria.
func notApplicableReason(m catalog.Metric, segment string, merged domain.MetricSet, value *float64) string {
	if m.NegativeEquityCheck && hasNonPositiveEquity(merged) {
		return m.NotApplicable[originAsset]
	}
	if value == nil && isSentinelSegment(m, segment) {
		return m.NotApplicable[originSector]
	}
	return ""
}

func isSentinelSegment(m catalog.Metric, segment string) bool {
	return slices.Contains(m.SentinelSegments, segment)
}

func hasNonPositiveEquity(merged domain.MetricSet) bool {
	o, ok := merged[equityID]
	return ok && o.Value != nil && *o.Value <= 0
}

func sortedIDs(set domain.MetricSet) []domain.MetricID {
	return slices.Sorted(maps.Keys(set))
}
