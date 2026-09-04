package store

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sort"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store/gen"
)

// Abaixo deste tamanho a distribuição do grupo não diz nada, e a comparação
// sobe um nível na taxonomia.
const MinPeerGroup = 5

const (
	levelSegment   = "segmento"
	levelSubsector = "subsetor"
	levelSector    = "setor"
	levelMarket    = "mercado"
	marketGroupKey = "mercado"
)

type MetricRule struct {
	MetricID         domain.MetricID
	ExcludeNegative  bool
	SentinelSegments []string
}

type AssetPercentile struct {
	MetricID         domain.MetricID
	Percentile       float64
	N                int
	FellBackToMarket bool
}

type peerAsset struct {
	assetID   int64
	class     domain.AssetClass
	sector    string
	subsector string
	segment   string
	level     string
	key       string
	n         int
}

// RecomputeSectorStats roda inteiro numa transação: uma falha no meio nunca
// deixa metade das estatísticas nova e metade velha.
func (db *DB) RecomputeSectorStats(ctx context.Context, rules []MetricRule, now time.Time) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin recompute sector stats: %w", err)
	}
	defer tx.Rollback()

	q := db.q.WithTx(tx)

	population, err := resolvePeerGroups(ctx, q)
	if err != nil {
		return err
	}
	if err := q.ClearSectorStats(ctx); err != nil {
		return fmt.Errorf("clear sector stats: %w", err)
	}
	if err := q.ClearAssetPercentiles(ctx); err != nil {
		return fmt.Errorf("clear asset percentiles: %w", err)
	}

	byID := make(map[int64]peerAsset, len(population))
	for _, p := range population {
		byID[p.assetID] = p
	}

	for _, rule := range rules {
		if err := materializeMetric(ctx, q, rule, byID, now); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit recompute sector stats: %w", err)
	}
	return nil
}

func resolvePeerGroups(ctx context.Context, q *gen.Queries) ([]peerAsset, error) {
	if err := q.ResetPeerGroups(ctx); err != nil {
		return nil, fmt.Errorf("reset peer groups: %w", err)
	}

	rows, err := q.ListPeerGroupPopulation(ctx)
	if err != nil {
		return nil, fmt.Errorf("list peer group population: %w", err)
	}

	population := make([]peerAsset, 0, len(rows))
	for _, r := range rows {
		population = append(population, peerAsset{
			assetID:   r.AssetID,
			class:     r.Class,
			sector:    deref(r.Sector),
			subsector: deref(r.Subsector),
			segment:   deref(r.Segment),
		})
	}

	bySegment := map[[4]string]int{}
	bySubsector := map[[3]string]int{}
	bySector := map[[2]string]int{}
	byClass := map[domain.AssetClass]int{}
	for _, p := range population {
		bySegment[segmentKey(p)]++
		bySubsector[subsectorKey(p)]++
		bySector[sectorKey(p)]++
		byClass[p.class]++
	}

	for i, p := range population {
		level, key, n := cascade(p, bySegment, bySubsector, bySector, byClass)
		population[i].level, population[i].key, population[i].n = level, key, n

		if err := q.UpdateAssetPeerGroup(ctx, gen.UpdateAssetPeerGroupParams{
			PeerGroupLevel: nullString(level),
			PeerGroupKey:   nullString(key),
			PeerGroupN:     int64Ptr(n),
			AssetID:        p.assetID,
		}); err != nil {
			return nil, fmt.Errorf("update peer group %d: %w", p.assetID, err)
		}
	}
	return population, nil
}

func cascade(p peerAsset, bySegment map[[4]string]int, bySubsector map[[3]string]int, bySector map[[2]string]int, byClass map[domain.AssetClass]int) (level, key string, n int) {
	if c := bySegment[segmentKey(p)]; c >= MinPeerGroup && p.segment != "" {
		return levelSegment, p.segment, c
	}
	if c := bySubsector[subsectorKey(p)]; c >= MinPeerGroup && p.subsector != "" {
		return levelSubsector, p.subsector, c
	}
	if c := bySector[sectorKey(p)]; c >= MinPeerGroup {
		return levelSector, p.sector, c
	}
	return levelMarket, marketGroupKey, byClass[p.class]
}

func segmentKey(p peerAsset) [4]string {
	return [4]string{string(p.class), p.sector, p.subsector, p.segment}
}

func subsectorKey(p peerAsset) [3]string {
	return [3]string{string(p.class), p.sector, p.subsector}
}

func sectorKey(p peerAsset) [2]string {
	return [2]string{string(p.class), p.sector}
}

type groupID struct {
	level string
	key   string
	class domain.AssetClass
}

func materializeMetric(ctx context.Context, q *gen.Queries, rule MetricRule, byID map[int64]peerAsset, now time.Time) error {
	rows, err := q.LatestMetricValuesForActive(ctx, rule.MetricID)
	if err != nil {
		return fmt.Errorf("latest values %s: %w", rule.MetricID, err)
	}

	eligible := make(map[int64]float64, len(rows))
	byGroup := map[groupID][]float64{}
	byMarket := map[domain.AssetClass][]float64{}

	for _, r := range rows {
		asset, ok := byID[r.AssetID]
		if !ok || r.Value == nil || !rule.accepts(asset, *r.Value) {
			continue
		}
		eligible[r.AssetID] = *r.Value
		byGroup[groupID{asset.level, asset.key, asset.class}] = append(
			byGroup[groupID{asset.level, asset.key, asset.class}], *r.Value)
		byMarket[asset.class] = append(byMarket[asset.class], *r.Value)
	}

	for g, values := range byGroup {
		if len(values) < MinPeerGroup {
			continue
		}
		sort.Float64s(values)
		if err := insertStat(ctx, q, g, rule.MetricID, values, now); err != nil {
			return err
		}
	}
	for class, values := range byMarket {
		if len(values) < MinPeerGroup {
			continue
		}
		sort.Float64s(values)
		g := groupID{levelMarket, marketGroupKey, class}
		if _, taken := byGroup[g]; taken {
			continue
		}
		if err := insertStat(ctx, q, g, rule.MetricID, values, now); err != nil {
			return err
		}
	}

	for assetID, value := range eligible {
		asset := byID[assetID]
		g := groupID{asset.level, asset.key, asset.class}

		values, fellBack := byGroup[g], false
		if len(values) < MinPeerGroup {
			values, fellBack = byMarket[asset.class], true
			g = groupID{levelMarket, marketGroupKey, asset.class}
		}
		if len(values) < MinPeerGroup {
			continue
		}

		if err := q.InsertAssetPercentile(ctx, gen.InsertAssetPercentileParams{
			AssetID:          assetID,
			MetricID:         string(rule.MetricID),
			Percentile:       percentileOf(values, value),
			GroupLevel:       g.level,
			GroupKey:         g.key,
			N:                int64(len(values)),
			FellBackToMarket: boolToInt(fellBack),
			ComputedAt:       now,
		}); err != nil {
			return fmt.Errorf("insert percentile %d/%s: %w", assetID, rule.MetricID, err)
		}
	}
	return nil
}

func (r MetricRule) accepts(asset peerAsset, value float64) bool {
	if r.ExcludeNegative && value < 0 {
		return false
	}
	return !(value == 0 && slices.Contains(r.SentinelSegments, asset.segment))
}

func insertStat(ctx context.Context, q *gen.Queries, g groupID, metric domain.MetricID, sorted []float64, now time.Time) error {
	err := q.InsertSectorStat(ctx, gen.InsertSectorStatParams{
		GroupLevel: g.level,
		GroupKey:   g.key,
		Class:      string(g.class),
		MetricID:   string(metric),
		N:          int64(len(sorted)),
		P10:        quantile(sorted, 0.10),
		P25:        quantile(sorted, 0.25),
		Median:     quantile(sorted, 0.50),
		P75:        quantile(sorted, 0.75),
		P90:        quantile(sorted, 0.90),
		ComputedAt: now,
	})
	if err != nil {
		return fmt.Errorf("insert sector stat %s/%s: %w", g.key, metric, err)
	}
	return nil
}

// Posto mais próximo, sem interpolação: o valor devolvido é sempre um número
// que algum papel real tem.
func quantile(sorted []float64, p float64) *float64 {
	if len(sorted) == 0 {
		return nil
	}
	idx := int(math.Ceil(p * float64(len(sorted))))
	idx = max(1, min(idx, len(sorted)))
	v := sorted[idx-1]
	return &v
}

// CUME_DIST, não PERCENT_RANK: o pior papel da amostra fica em 1/n, não em
// zero, que se leria como "não tem posição".
func percentileOf(sorted []float64, value float64) float64 {
	count := 0
	for _, v := range sorted {
		if v <= value {
			count++
		}
	}
	return float64(count) / float64(len(sorted))
}

func (db *DB) GetAssetPercentiles(ctx context.Context, assetID int64) ([]AssetPercentile, error) {
	rows, err := db.q.GetAssetPercentiles(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("get asset percentiles %d: %w", assetID, err)
	}

	out := make([]AssetPercentile, 0, len(rows))
	for _, r := range rows {
		out = append(out, AssetPercentile{
			MetricID:         domain.MetricID(r.MetricID),
			Percentile:       r.Percentile,
			N:                int(r.N),
			FellBackToMarket: r.FellBackToMarket != 0,
		})
	}
	return out, nil
}

func int64Ptr(n int) *int64 {
	v := int64(n)
	return &v
}

func boolToInt(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

type SectorCount struct {
	Name string
	N    int
}

func (db *DB) ListSectorCounts(ctx context.Context, class domain.AssetClass) ([]SectorCount, error) {
	rows, err := db.q.ListSectorCounts(ctx, class)
	if err != nil {
		return nil, fmt.Errorf("list sector counts %s: %w", class, err)
	}
	out := make([]SectorCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, SectorCount{Name: deref(r.Sector), N: int(r.N)})
	}
	return out, nil
}

func (db *DB) ListSubsectorCounts(ctx context.Context, class domain.AssetClass, sector string) ([]SectorCount, error) {
	rows, err := db.q.ListSubsectorCounts(ctx, gen.ListSubsectorCountsParams{
		Class: class, Sector: nullString(sector),
	})
	if err != nil {
		return nil, fmt.Errorf("list subsector counts %s: %w", sector, err)
	}
	out := make([]SectorCount, 0, len(rows))
	for _, r := range rows {
		out = append(out, SectorCount{Name: deref(r.Subsector), N: int(r.N)})
	}
	return out, nil
}

func (db *DB) SectorExists(ctx context.Context, class domain.AssetClass, sector string) (bool, error) {
	n, err := db.q.SectorExists(ctx, gen.SectorExistsParams{Class: class, Sector: nullString(sector)})
	if err != nil {
		return false, fmt.Errorf("sector exists %s: %w", sector, err)
	}
	return n > 0, nil
}
