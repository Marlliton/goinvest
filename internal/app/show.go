// Package app é a fronteira que cmd e, nas fases seguintes, a TUI enxergam.
package app

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/derive"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
)

var ErrNoData = errors.New("nenhum dado local. Rode 'goinvest sync' primeiro")

const stalenessThreshold = 7 * 24 * time.Hour

type HeaderView struct {
	ReferenceAt *time.Time
	FetchedAt   time.Time
	// Resolvida aqui para que a renderização não precise de relógio.
	Age   time.Duration
	Stale bool
}

type LineView struct {
	MetricID domain.MetricID
	Label    string
	Value    *float64
	Unit     domain.Unit
	Derived  bool
	Formula  string
}

type BlockView struct {
	Label string
	Lines []LineView
}

type Report struct {
	Ticker string
	Class  domain.AssetClass
	Header HeaderView
	Blocks []BlockView
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

	return Report{
		Ticker: asset.Ticker,
		Class:  asset.Class,
		Header: header(collected, now),
		Blocks: blocks(cat, asset.Class, merged),
	}, nil
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

func blocks(cat *catalog.Catalog, class domain.AssetClass, merged domain.MetricSet) []BlockView {
	applicable := cat.MetricsFor(class)

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
				continue
			}
			view.Lines = append(view.Lines, LineView{
				MetricID: m.ID,
				Label:    m.Label,
				Value:    o.Value,
				Unit:     m.Unit,
				Derived:  m.Derived,
				Formula:  m.Formula,
			})
		}
		if len(view.Lines) > 0 {
			out = append(out, view)
		}
	}
	return out
}

func sortedIDs(set domain.MetricSet) []domain.MetricID {
	return slices.Sorted(maps.Keys(set))
}
