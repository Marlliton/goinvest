package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
)

// UpsertAsset atualiza classe e nome. asset é dimensão, não fato histórico:
// aqui sobrescrever é correto, diferente de observation.
func (db *DB) UpsertAsset(ctx context.Context, ticker string, class domain.AssetClass, name string, updatedAt time.Time) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO asset (ticker, class, name, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(ticker) DO UPDATE SET
			class      = excluded.class,
			name       = excluded.name,
			updated_at = excluded.updated_at`,
		ticker, string(class), name, updatedAt)
	if err != nil {
		return fmt.Errorf("upsert asset %s: %w", ticker, err)
	}
	return nil
}

// GetAsset devolve found=false para ticker inexistente. Ausência não é erro:
// distinguir "nunca sincronizado" de "não existe na B3" é escopo da Fase 2.
func (db *DB) GetAsset(ctx context.Context, ticker string) (domain.Asset, bool, error) {
	var (
		a    domain.Asset
		cls  string
		name sql.NullString
	)
	err := db.QueryRowContext(ctx,
		`SELECT ticker, class, name, updated_at FROM asset WHERE ticker = ?`, ticker).
		Scan(&a.Ticker, &cls, &name, &a.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Asset{}, false, nil
	}
	if err != nil {
		return domain.Asset{}, false, fmt.Errorf("get asset %s: %w", ticker, err)
	}
	a.Class = domain.AssetClass(cls)
	a.Name = name.String
	return a, true, nil
}

func (db *DB) StartRun(ctx context.Context, source string) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO collection_run (source, started_at, status) VALUES (?, ?, 'running')`,
		source, time.Now().UTC())
	if err != nil {
		return 0, fmt.Errorf("start run %s: %w", source, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("start run %s: %w", source, err)
	}
	return id, nil
}

// FinishRun é a única escrita destrutiva legítima do schema: fecha a linha de
// collection_run aberta por StartRun.
func (db *DB) FinishRun(ctx context.Context, runID int64, status string, nObs int, errMsg string) error {
	var msg any
	if errMsg != "" {
		msg = errMsg
	}
	_, err := db.ExecContext(ctx,
		`UPDATE collection_run SET finished_at = ?, status = ?, n_obs = ?, error = ? WHERE id = ?`,
		time.Now().UTC(), status, nObs, msg, runID)
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

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO observation
			(ticker, metric_id, period_kind, period_end, value, unit, source, reference_at, fetched_at, run_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare insert observation: %w", err)
	}
	defer stmt.Close()

	for _, o := range obs {
		var value, referenceAt any
		if o.Value != nil {
			value = *o.Value
		}
		if o.ReferenceAt != nil {
			referenceAt = *o.ReferenceAt
		}
		if _, err := stmt.ExecContext(ctx,
			o.Ticker, string(o.Metric), o.PeriodKind, o.PeriodEnd,
			value, string(o.Unit), o.Source, referenceAt, o.FetchedAt, runID,
		); err != nil {
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
	rows, err := db.QueryContext(ctx, `
		SELECT metric_id, period_kind, period_end, value, unit, source, reference_at, fetched_at, run_id
		FROM (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY metric_id, period_kind
				ORDER BY period_end DESC, fetched_at DESC) AS rn
			FROM observation
			WHERE ticker = ?
		)
		WHERE rn = 1`, ticker)
	if err != nil {
		return nil, fmt.Errorf("latest metrics %s: %w", ticker, err)
	}
	defer rows.Close()

	set := domain.MetricSet{}
	for rows.Next() {
		var (
			o           domain.Observation
			metricID    string
			unit        string
			value       sql.NullFloat64
			referenceAt sql.NullTime
			runID       sql.NullInt64
		)
		if err := rows.Scan(&metricID, &o.PeriodKind, &o.PeriodEnd, &value, &unit,
			&o.Source, &referenceAt, &o.FetchedAt, &runID); err != nil {
			return nil, fmt.Errorf("scan latest metric %s: %w", ticker, err)
		}

		o.Ticker = ticker
		o.Metric = domain.MetricID(metricID)
		o.Unit = domain.Unit(unit)
		if value.Valid {
			v := value.Float64
			o.Value = &v
		}
		if referenceAt.Valid {
			t := referenceAt.Time
			o.ReferenceAt = &t
		}
		o.RunID = runID.Int64
		set[o.Metric] = o
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("latest metrics %s: %w", ticker, err)
	}
	return set, nil
}
