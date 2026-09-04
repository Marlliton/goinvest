package store

import (
	"context"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store/gen"
)

func (db *DB) StartRun(ctx context.Context, source string) (int64, error) {
	id, err := db.q.StartRun(ctx, gen.StartRunParams{
		Source:    source,
		StartedAt: time.Now().UTC(),
	})
	if err != nil {
		return 0, fmt.Errorf("start run %s: %w", source, err)
	}
	return id, nil
}

// FinishRun é a única escrita destrutiva legítima do schema: fecha a linha de
// collection_run aberta por StartRun.
func (db *DB) FinishRun(ctx context.Context, runID int64, status string, nObs int, errMsg string) error {
	now := time.Now().UTC()
	n := int64(nObs)
	err := db.q.FinishRun(ctx, gen.FinishRunParams{
		FinishedAt: &now,
		Status:     status,
		NObs:       &n,
		Error:      nullString(errMsg),
		ID:         runID,
	})
	if err != nil {
		return fmt.Errorf("finish run %d: %w", runID, err)
	}
	return nil
}

func (db *DB) InsertObservations(ctx context.Context, runID int64, obs []domain.Observation) error {
	if len(obs) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert observations: %w", err)
	}
	defer tx.Rollback()

	byTicker, err := db.assetIDsByTicker(ctx)
	if err != nil {
		return err
	}

	q := db.q.WithTx(tx)
	for _, o := range obs {
		assetID, ok := byTicker[o.Ticker]
		if !ok {
			return fmt.Errorf("insert observation %s/%s: grave o asset antes da observação", o.Ticker, o.Metric)
		}
		if err := q.InsertObservation(ctx, gen.InsertObservationParams{
			AssetID:     assetID,
			MetricID:    o.Metric,
			PeriodKind:  o.PeriodKind,
			PeriodEnd:   o.PeriodEnd,
			Value:       o.Value,
			Unit:        o.Unit,
			Source:      o.Source,
			ReferenceAt: o.ReferenceAt,
			FetchedAt:   o.FetchedAt,
			RunID:       &runID,
		}); err != nil {
			return fmt.Errorf("insert observation %s/%s: %w", o.Ticker, o.Metric, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert observations: %w", err)
	}
	return nil
}

// LatestMetrics devolve a observação corrente de cada métrica do ativo.
// Métrica nunca coletada não vira chave no mapa; métrica coletada sem valor
// vira chave com Value nil. O ticker só preenche as linhas devolvidas: a
// consulta em si é por asset_id.
func (db *DB) LatestMetrics(ctx context.Context, assetID int64, ticker string) (domain.MetricSet, error) {
	rows, err := db.q.LatestMetrics(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("latest metrics %s: %w", ticker, err)
	}

	set := make(domain.MetricSet, len(rows))
	for _, r := range rows {
		o := domain.Observation{
			Ticker:      ticker,
			Metric:      r.MetricID,
			PeriodKind:  r.PeriodKind,
			PeriodEnd:   r.PeriodEnd,
			Value:       r.Value,
			Unit:        r.Unit,
			Source:      r.Source,
			ReferenceAt: r.ReferenceAt,
			FetchedAt:   r.FetchedAt,
		}
		if r.RunID != nil {
			o.RunID = *r.RunID
		}
		set[o.Metric] = o
	}
	return set, nil
}

func (db *DB) assetIDsByTicker(ctx context.Context) (map[string]int64, error) {
	rows, err := db.q.ListAssetIDsByTicker(ctx)
	if err != nil {
		return nil, fmt.Errorf("list asset ids: %w", err)
	}
	byTicker := make(map[string]int64, len(rows))
	for _, r := range rows {
		byTicker[r.Ticker] = r.AssetID
	}
	return byTicker, nil
}
