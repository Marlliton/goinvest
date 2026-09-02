// Package collect orquestra as fontes bulk. Cada classe tem seu próprio
// collection_run: a falha de uma nunca interrompe a coleta da outra.
package collect

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/store"
)

const (
	StatusOK      = "ok"
	StatusPartial = "partial"
)

type SourceResult struct {
	Source     string
	AssetCount int
	Status     string
	Reason     string
	Duration   time.Duration
}

type Report struct {
	Stocks SourceResult
	FIIs   SourceResult
}

type Config struct {
	Providers map[domain.AssetClass]provider.UniverseProvider
	DB        *store.DB
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

	return Report{
		Stocks: collectClass(ctx, cfg, domain.ClassStock),
		FIIs:   collectClass(ctx, cfg, domain.ClassFII),
	}, nil
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
	seen := make(map[string]struct{})
	for _, o := range obs {
		if _, dup := seen[o.Ticker]; dup {
			continue
		}
		seen[o.Ticker] = struct{}{}
		// A observação referencia asset(ticker): sem o upsert antes, a
		// inserção em lote inteira falha na chave estrangeira.
		if err := cfg.DB.UpsertAsset(ctx, o.Ticker, class, "", at); err != nil {
			return 0, 0, err
		}
	}

	if err := cfg.DB.InsertObservations(ctx, runID, obs); err != nil {
		return 0, 0, err
	}
	return len(seen), len(obs), nil
}
