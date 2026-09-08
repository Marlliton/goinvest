package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/store/gen"
)

// macro_rate guarda o valor corrente de cada taxa, não a série histórica.
const selicRateID = "selic"

func (db *DB) GetSelic(ctx context.Context) (value *float64, referenceAt, fetchedAt *time.Time, found bool, err error) {
	row, err := db.q.GetMacroRate(ctx, selicRateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("get selic: %w", err)
	}
	return &row.Value, &row.ReferenceAt, &row.FetchedAt, true, nil
}

func (db *DB) PutSelic(ctx context.Context, value float64, referenceAt, fetchedAt time.Time) error {
	err := db.q.UpsertMacroRate(ctx, gen.UpsertMacroRateParams{
		ID:          selicRateID,
		Value:       value,
		ReferenceAt: referenceAt,
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		return fmt.Errorf("put selic: %w", err)
	}
	return nil
}
