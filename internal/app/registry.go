package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/collect"
	"github.com/marlliton/goinvest/internal/registry"
)

type RegistryConfig = registry.Config

func Registry(ctx context.Context, cfg RegistryConfig) (registry.Report, error) {
	return registry.Run(ctx, cfg)
}

type RegistryFIIConfig = registry.FIIConfig

func RegistryFII(ctx context.Context, cfg RegistryFIIConfig) (registry.Report, error) {
	return registry.RunFII(ctx, cfg)
}

var ErrRegistryCancelled = errors.New("cadastro interrompido pelo usuário")

type RegistryAllConfig struct {
	Stocks  RegistryConfig
	FIIs    RegistryFIIConfig
	Catalog *catalog.Catalog
	Now     func() time.Time
}

type RegistryAllReport struct {
	Stocks          registry.Report
	StocksCancelled bool
	FIIs            registry.Report
	FIIsCancelled   bool
	FIIsSkipped     bool
}

// RegistryAll orquestra os dois estágios de cadastro e o recálculo da
// referência setorial. O recálculo roda sempre que houver catálogo, mesmo
// quando um estágio falhou de verdade: só um cancelamento pelo usuário (via
// context) pula o estágio de FIIs, uma falha real não deveria deixar a
// referência velha em silêncio.
func RegistryAll(ctx context.Context, cfg RegistryAllConfig) (RegistryAllReport, error) {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	var result RegistryAllReport

	stocksReport, stocksRunErr := Registry(ctx, cfg.Stocks)
	stocksOutcome, stocksCancelled, stocksErr := stageOutcome(ctx, stocksReport, stocksRunErr)
	result.Stocks = stocksOutcome
	result.StocksCancelled = stocksCancelled

	var fiiErr error
	switch {
	case stocksCancelled || stocksErr != nil:
		result.FIIsSkipped = true
	default:
		fiiReport, fiiRunErr := RegistryFII(ctx, cfg.FIIs)
		fiiOutcome, fiiCancelled, outcomeErr := stageOutcome(ctx, fiiReport, fiiRunErr)
		result.FIIs = fiiOutcome
		result.FIIsCancelled = fiiCancelled
		fiiErr = outcomeErr
	}

	var recomputeErr error
	if cfg.Stocks.DB != nil && cfg.Catalog != nil {
		recomputeErr = cfg.Stocks.DB.RecomputeSectorStats(
			context.WithoutCancel(ctx), collect.MetricRules(cfg.Catalog), cfg.Now())
	}

	switch {
	case stocksErr != nil:
		return result, stocksErr
	case fiiErr != nil:
		return result, fiiErr
	case recomputeErr != nil:
		return result, fmt.Errorf("recalcular referência setorial: %w", recomputeErr)
	case stocksCancelled || result.FIIsCancelled:
		return result, ErrRegistryCancelled
	default:
		return result, nil
	}
}

// stageOutcome separa cancelamento do erro cru devolvido pela fonte: um
// Ctrl-C durante a coleta propaga context.Canceled envolto na URL da
// requisição em andamento, e esse texto nunca deve chegar ao chamador.
func stageOutcome(ctx context.Context, report registry.Report, err error) (registry.Report, bool, error) {
	if err != nil {
		if ctx.Err() != nil {
			return registry.Report{}, true, nil
		}
		return registry.Report{}, false, err
	}
	return report, report.Cancelled, nil
}
