// Package app é a fronteira que cmd e, nas fases seguintes, a TUI enxergam.
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
	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
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
	MetricID         domain.MetricID
	Label            string
	Value            *float64
	Unit             domain.Unit
	Derived          bool
	Formula          string
	Percentile       *float64
	PeerN            *int
	FellBackToMarket bool
	SelicDelta       *float64
	ReferenceAt      *time.Time
	// Preenchido só quando a métrica é estruturalmente não aplicável. Ausência
	// comum ("a fonte não informou") continua sendo Value nil com motivo vazio.
	NotApplicableReason string
}

type BlockView struct {
	Label string
	Lines []LineView
}

// Bazin é família, não linha: teto, ágio/deságio, anos usados e a lista anual
// contam a mesma história e não cabem no esquema de LineView.
type BazinView struct {
	Ceiling             float64
	CurrentPrice        float64
	PremiumDiscount     float64
	YearsUsed           int
	Years               []bazin.YearlyDividend
	AtypicalYears       []int
	NotApplicableReason string
}

type Report struct {
	Ticker string
	Class  domain.AssetClass
	Header HeaderView
	Blocks []BlockView
	Bazin  *BazinView
}

func Show(ctx context.Context, db *store.DB, cat *catalog.Catalog, ticker string, now func() time.Time) (Report, error) {
	asset, found, err := db.GetAsset(ctx, ticker)
	if err != nil {
		return Report{}, err
	}
	if !found {
		return Report{}, ErrNoData
	}

	collected, err := db.LatestMetrics(ctx, asset.AssetID, asset.Ticker)
	if err != nil {
		return Report{}, err
	}
	// Cadastrado e nunca coletado pede do usuário a mesma ação que ausente.
	if len(collected) == 0 {
		return Report{}, ErrNoData
	}

	merged := maps.Clone(collected)
	for id, o := range derive.Compute(collected) {
		if _, taken := merged[id]; !taken {
			merged[id] = o
		}
	}

	total, withSector, err := db.SectorCoverage(ctx, asset.Class)
	if err != nil {
		return Report{}, err
	}

	h := header(collected, now)
	h.Inactive = !asset.IsActive
	h.LastLiquidAt = asset.LastLiquidAt
	h.Sector, h.Subsector, h.Segment = asset.Sector, asset.Subsector, asset.Segment
	h.TotalInClass = total
	h.IncompleteRegistry = total - withSector

	if rate, referenceAt, _, found, err := db.GetSelic(ctx); err != nil {
		return Report{}, err
	} else if found {
		h.SelicRate, h.SelicAt = rate, referenceAt
	}

	hasDetail, err := db.HasDetail(ctx, asset.AssetID)
	if err != nil {
		return Report{}, err
	}

	var percentiles map[domain.MetricID]store.AssetPercentile
	if asset.IsActive {
		h.PeerGroupLabel, h.PeerGroupN = peerGroup(asset)
		percentiles, err = assetPercentiles(ctx, db, asset.AssetID)
		if err != nil {
			return Report{}, err
		}
	}

	events, err := db.ListDividendEvents(ctx, asset.AssetID)
	if err != nil {
		return Report{}, err
	}

	return Report{
		Ticker: asset.Ticker,
		Class:  asset.Class,
		Header: h,
		Blocks: blocks(cat, asset, merged, percentiles, h, hasDetail),
		Bazin:  bazinView(events, merged, h, asset.Ticker, now()),
	}, nil
}

func bazinView(events []domain.DividendEvent, merged domain.MetricSet, h HeaderView, ticker string, now time.Time) *BazinView {
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
	// O teto nunca sai sozinho: sem a âncora ao lado, um R$ 20,00 de teto se lê
	// como veredito, e é justamente a comparação com a renda fixa que decide se
	// o dividendo compensa.
	if h.SelicRate == nil {
		return &BazinView{NotApplicableReason: "Selic desconhecida; rode 'goinvest sync'"}
	}
	if _, hasYield := presentValue(merged, dividendYieldID); !hasYield {
		return &BazinView{NotApplicableReason: "DY não coletado; o teto não sai sem o DY−Selic ao lado"}
	}
	if result.Ceiling <= 0 {
		return &BazinView{NotApplicableReason: "os proventos da janela somam zero"}
	}

	return &BazinView{
		Ceiling:         result.Ceiling,
		CurrentPrice:    price,
		PremiumDiscount: (price - result.Ceiling) / result.Ceiling,
		YearsUsed:       result.YearsAvailable,
		Years:           result.Years,
		AtypicalYears:   result.AtypicalYears,
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

// Nulo continua nulo: preencher com FetchedAt afirmaria uma competência que a
// fonte nunca informou.
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

func blocks(cat *catalog.Catalog, asset domain.Asset, merged domain.MetricSet, percentiles map[domain.MetricID]store.AssetPercentile, h HeaderView, hasDetail bool) []BlockView {
	applicable := cat.MetricsFor(asset.Class)

	out := make([]BlockView, 0, len(cat.Blocks))
	for _, b := range cat.BlocksOrdered() {
		view := BlockView{Label: b.Label}
		for _, m := range applicable {
			if m.Block != b.ID {
				continue
			}
			// Nunca coletada some da tela; coletada sem valor vira "—".
			o, ok := merged[m.ID]
			if !ok {
				// Sem o documento em mãos não dá para afirmar que a fonte não
				// publica: seria vender palpite como conclusão.
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

const equityID = domain.MetricID("patrim_liq")

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

// A checagem de patrimônio prevalece sobre o número: o múltiplo é calculável,
// mas compara preço com um patrimônio que não existe.
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
