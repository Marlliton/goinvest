package store

import (
	"context"
	"fmt"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store/gen"
)

func (db *DB) InsertDividendEvents(ctx context.Context, events []domain.DividendEvent) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin insert dividend events: %w", err)
	}
	defer tx.Rollback()

	byTicker, err := db.assetIDsByTicker(ctx)
	if err != nil {
		return err
	}

	q := db.q.WithTx(tx)
	for _, e := range events {
		assetID, ok := byTicker[e.Ticker]
		if !ok {
			return fmt.Errorf("insert dividend event %s: grave o asset antes do provento", e.Ticker)
		}
		if err := q.InsertDividendEvent(ctx, gen.InsertDividendEventParams{
			AssetID:      assetID,
			ExDate:       e.ExDate,
			PaymentDate:  e.PaymentDate,
			Type:         string(e.Type),
			TypeRaw:      e.TypeRaw,
			ValueRaw:     e.ValuePerShareRaw,
			SharesFactor: e.SharesFactor,
			Source:       e.Source,
			FetchedAt:    e.FetchedAt,
		}); err != nil {
			return fmt.Errorf("insert dividend event %s/%s: %w", e.Ticker, e.ExDate.Format("2006-01-02"), err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit insert dividend events: %w", err)
	}
	return nil
}

func (db *DB) ListDividendEvents(ctx context.Context, assetID int64) ([]domain.DividendEvent, error) {
	rows, err := db.q.ListDividendEvents(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("list dividend events %d: %w", assetID, err)
	}

	out := make([]domain.DividendEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.DividendEvent{
			ExDate:           r.ExDate,
			PaymentDate:      r.PaymentDate,
			Type:             domain.DividendType(r.Type),
			TypeRaw:          r.TypeRaw,
			ValuePerShareRaw: r.ValueRaw,
			SharesFactor:     r.SharesFactor,
			Source:           r.Source,
			FetchedAt:        r.FetchedAt,
		})
	}
	return out, nil
}
