package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store/gen"
)

// UpsertAsset atualiza classe e nome. asset é dimensão, não fato histórico:
// aqui sobrescrever é correto, diferente de observation.
func (db *DB) UpsertAsset(ctx context.Context, ticker string, class domain.AssetClass, name string, updatedAt time.Time) error {
	err := db.q.UpsertAsset(ctx, gen.UpsertAssetParams{
		Ticker:    ticker,
		Class:     class,
		Name:      nullString(name),
		UpdatedAt: updatedAt,
	})
	if err != nil {
		return fmt.Errorf("upsert asset %s: %w", ticker, err)
	}
	return nil
}

// GetAsset devolve found=false para ticker inexistente. Ausência não é erro:
// distinguir "nunca sincronizado" de "não existe na B3" é escopo da Fase 2.
func (db *DB) GetAsset(ctx context.Context, ticker string) (domain.Asset, bool, error) {
	a, err := db.q.GetAsset(ctx, ticker)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Asset{}, false, nil
	}
	if err != nil {
		return domain.Asset{}, false, fmt.Errorf("get asset %s: %w", ticker, err)
	}
	return domain.Asset{
		Ticker:    a.Ticker,
		Class:     a.Class,
		Name:      deref(a.Name),
		UpdatedAt: a.UpdatedAt,
	}, true, nil
}

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

	q := db.q.WithTx(tx)
	for _, o := range obs {
		if err := q.InsertObservation(ctx, gen.InsertObservationParams{
			Ticker:      o.Ticker,
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

// LatestMetrics devolve a observação corrente de cada métrica do ticker.
// Métrica nunca coletada não vira chave no mapa; métrica coletada sem valor
// vira chave com Value nil.
func (db *DB) LatestMetrics(ctx context.Context, ticker string) (domain.MetricSet, error) {
	rows, err := db.q.LatestMetrics(ctx, ticker)
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
