// Package collect orquestra as fontes bulk. Cada classe tem seu próprio
// collection_run: a falha de uma nunca interrompe a coleta da outra.
package collect

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/store"
)

const (
	StatusOK      = "ok"
	StatusPartial = "partial"
)

// Volume médio diário em reais.
const (
	minLiquidityStock = 1_000_000.0
	minLiquidityFII   = 200_000.0
)

type SourceResult struct {
	Source     string
	AssetCount int
	Status     string
	Reason     string
	Duration   time.Duration
}

type Report struct {
	Stocks      SourceResult
	FIIs        SourceResult
	SectorStats string
}

type Config struct {
	Providers map[domain.AssetClass]provider.UniverseProvider
	DB        *store.DB
	Catalog   *catalog.Catalog
	Force     bool
	Now       func() time.Time
}

// Sync coleta sempre as duas classes e só devolve erro para o que impede
// qualquer coleta. Uma fonte que falha vira Status partial no SourceResult dela.
func Sync(ctx context.Context, cfg Config) (Report, error) {
	if cfg.DB == nil {
		return Report{}, errors.New("collect: db is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	report := Report{
		Stocks: collectClass(ctx, cfg, domain.ClassStock),
		FIIs:   collectClass(ctx, cfg, domain.ClassFII),
	}

	if cfg.Catalog != nil {
		if err := cfg.DB.RecomputeSectorStats(ctx, metricRules(cfg.Catalog), cfg.Now()); err != nil {
			report.SectorStats = err.Error()
		}
	}
	return report, nil
}

func metricRules(cat *catalog.Catalog) []store.MetricRule {
	rules := make([]store.MetricRule, 0, len(cat.Metrics))
	for _, m := range cat.Metrics {
		if !m.Percentile {
			continue
		}
		rules = append(rules, store.MetricRule{
			MetricID:         m.ID,
			ExcludeNegative:  m.ExcludeNegative,
			SentinelSegments: m.SentinelSegments,
		})
	}
	return rules
}

func collectClass(ctx context.Context, cfg Config, class domain.AssetClass) SourceResult {
	started := cfg.Now()

	p, ok := cfg.Providers[class]
	if !ok {
		return SourceResult{
			Status: StatusPartial,
			Reason: fmt.Sprintf("nenhum provider registrado para a classe %s", class),
		}
	}

	res := SourceResult{Source: p.SourceID(class), Status: StatusOK}

	runID, err := cfg.DB.StartRun(ctx, res.Source)
	if err != nil {
		res.Status, res.Reason, res.Duration = StatusPartial, err.Error(), cfg.Now().Sub(started)
		return res
	}

	assets, observations, err := ingest(ctx, cfg, class, p, runID)
	if err != nil {
		res.Status, res.Reason = StatusPartial, err.Error()
	} else {
		res.AssetCount = assets
	}

	if err := cfg.DB.FinishRun(ctx, runID, res.Status, observations, res.Reason); err != nil {
		res.Status, res.Reason = StatusPartial, err.Error()
	}

	res.Duration = cfg.Now().Sub(started)
	return res
}

func ingest(ctx context.Context, cfg Config, class domain.AssetClass, p provider.UniverseProvider, runID int64) (assets, observations int, err error) {
	obs, err := p.Universe(ctx, class, cfg.Force)
	if err != nil {
		return 0, 0, err
	}

	at := cfg.Now()
	values := metricsByTicker(obs)
	seen := make(map[string]struct{})
	for _, o := range obs {
		if _, dup := seen[o.Ticker]; dup {
			continue
		}
		seen[o.Ticker] = struct{}{}
		// A observação referencia asset(asset_id): sem o upsert antes, a
		// inserção em lote inteira falha na chave estrangeira.
		if err := cfg.DB.UpsertAsset(ctx, o.Ticker, class, "", at); err != nil {
			return 0, 0, err
		}

		assetID, found, err := cfg.DB.AssetIDByTicker(ctx, o.Ticker)
		if err != nil {
			return 0, 0, err
		}
		if !found {
			continue
		}

		if err := cfg.DB.UpdateAssetLiquidity(ctx, assetID, isActive(class, values[o.Ticker]), at); err != nil {
			return 0, 0, err
		}

		if class != domain.ClassStock {
			continue
		}
		if err := cfg.DB.UpsertAssetAlias(ctx, identity.FractionalAlias(o.Ticker), assetID); err != nil {
			return 0, 0, err
		}
	}

	if err := cfg.DB.InsertObservations(ctx, runID, obs); err != nil {
		return 0, 0, err
	}
	return len(seen), len(obs), nil
}

func metricsByTicker(obs []domain.Observation) map[string]map[domain.MetricID]*float64 {
	values := make(map[string]map[domain.MetricID]*float64)
	for _, o := range obs {
		if values[o.Ticker] == nil {
			values[o.Ticker] = make(map[domain.MetricID]*float64, 2)
		}
		values[o.Ticker][o.Metric] = o.Value
	}
	return values
}

// Métrica ausente não marca nada: só um zero lido da fonte é evidência de que
// o papel não negocia.
func isActive(class domain.AssetClass, values map[domain.MetricID]*float64) bool {
	quote := values["cotacao"]
	liquidity := values[liquidityMetric(class)]

	if isZero(quote) || isZero(liquidity) {
		return false
	}
	return liquidity == nil || *liquidity >= minLiquidity(class)
}

func liquidityMetric(class domain.AssetClass) domain.MetricID {
	if class == domain.ClassFII {
		return "liquidez_fii"
	}
	return "liq_2meses"
}

func minLiquidity(class domain.AssetClass) float64 {
	if class == domain.ClassFII {
		return minLiquidityFII
	}
	return minLiquidityStock
}

func isZero(v *float64) bool { return v != nil && *v == 0 }
