package collect

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/store"
)

const deepSourceID = "fundamentus:detalhes"

type Progress struct {
	Done  int
	Total int
}

type TickerOutcome struct {
	Ticker string
	Status string
	Reason string
}

type DeepReport struct {
	Outcomes  []TickerOutcome
	Cancelled bool
}

type DeepConfig struct {
	DB         *store.DB
	Detail     provider.DetailProvider
	Tickers    []string
	Force      bool
	Now        func() time.Time
	OnProgress func(Progress)
}

func Deep(ctx context.Context, cfg DeepConfig) (DeepReport, error) {
	if cfg.DB == nil {
		return DeepReport{}, errors.New("collect: db is required")
	}
	if cfg.Detail == nil {
		return DeepReport{}, errors.New("collect: detail provider is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	runID, err := cfg.DB.StartRun(ctx, deepSourceID)
	if err != nil {
		return DeepReport{}, err
	}

	report := DeepReport{Outcomes: make([]TickerOutcome, 0, len(cfg.Tickers))}
	written := 0

	for _, ticker := range cfg.Tickers {
		if ctx.Err() != nil {
			report.Cancelled = true
			break
		}

		outcome, n := collectTicker(ctx, cfg, runID, ticker)
		// Cancelar no meio de um ticker não é falha dele: o resultado culparia a fonte pelo Ctrl-C.
		if ctx.Err() != nil {
			report.Cancelled = true
			break
		}

		written += n
		report.Outcomes = append(report.Outcomes, outcome)
		if cfg.OnProgress != nil {
			cfg.OnProgress(Progress{Done: len(report.Outcomes), Total: len(cfg.Tickers)})
		}
	}

	if err := cfg.DB.FinishRun(context.WithoutCancel(ctx), runID, deepStatus(report), written, ""); err != nil {
		return report, err
	}
	return report, nil
}

func collectTicker(ctx context.Context, cfg DeepConfig, runID int64, ticker string) (TickerOutcome, int) {
	fail := func(reason string) (TickerOutcome, int) {
		return TickerOutcome{Ticker: ticker, Status: StatusPartial, Reason: reason}, 0
	}

	if !identity.ValidTicker(ticker) {
		return fail("não é um código da B3: corrija a digitação")
	}

	asset, found, err := cfg.DB.GetAsset(ctx, ticker)
	if err != nil {
		return fail(err.Error())
	}
	if !found {
		return fail("sem cadastro local: rode 'goinvest sync'")
	}

	// A fonte não publica página do fracionário; GetAsset devolve o canônico.
	canonical := asset.Ticker

	metrics, err := cfg.Detail.Detail(ctx, canonical, asset.Class, cfg.Force)
	if err != nil {
		return fail(err.Error())
	}
	events, err := cfg.Detail.Dividends(ctx, canonical, asset.Class, cfg.Force)
	if err != nil {
		return fail(err.Error())
	}

	obs := observationsOf(metrics)
	if err := cfg.DB.InsertObservations(ctx, runID, obs); err != nil {
		return fail(err.Error())
	}
	if err := cfg.DB.InsertDividendEvents(ctx, events); err != nil {
		return fail(err.Error())
	}

	return TickerOutcome{Ticker: ticker, Status: StatusOK}, len(obs)
}

// Ordem de mapa é aleatória: a mensagem de erro mudaria entre execuções idênticas.
func observationsOf(set domain.MetricSet) []domain.Observation {
	obs := make([]domain.Observation, 0, len(set))
	for _, id := range slices.Sorted(maps.Keys(set)) {
		obs = append(obs, set[id])
	}
	return obs
}

func deepStatus(r DeepReport) string {
	if r.Cancelled {
		return StatusCancelled
	}
	for _, o := range r.Outcomes {
		if o.Status != StatusOK {
			return StatusPartial
		}
	}
	return StatusOK
}
