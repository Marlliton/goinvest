package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/store/gen"
)

const (
	cdiRateID            = "cdi"
	tesouroIPCA10yRateID = "tesouro_ipca_10a"
	focusIPCA12mRateID   = "ipca_focus_12m"
)

func (db *DB) GetCDI(ctx context.Context) (value *float64, referenceAt, fetchedAt *time.Time, found bool, err error) {
	row, err := db.q.GetMacroRate(ctx, cdiRateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("get cdi: %w", err)
	}
	return &row.Value, &row.ReferenceAt, &row.FetchedAt, true, nil
}

func (db *DB) PutCDI(ctx context.Context, value float64, referenceAt, fetchedAt time.Time) error {
	err := db.q.UpsertMacroRate(ctx, gen.UpsertMacroRateParams{
		ID:          cdiRateID,
		Value:       value,
		ReferenceAt: referenceAt,
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		return fmt.Errorf("put cdi: %w", err)
	}
	return nil
}

func (db *DB) GetTesouroIPCA10y(ctx context.Context) (value *float64, referenceAt, fetchedAt *time.Time, found bool, err error) {
	row, err := db.q.GetMacroRate(ctx, tesouroIPCA10yRateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("get tesouro ipca 10a: %w", err)
	}
	return &row.Value, &row.ReferenceAt, &row.FetchedAt, true, nil
}

func (db *DB) PutTesouroIPCA10y(ctx context.Context, value float64, referenceAt, fetchedAt time.Time) error {
	err := db.q.UpsertMacroRate(ctx, gen.UpsertMacroRateParams{
		ID:          tesouroIPCA10yRateID,
		Value:       value,
		ReferenceAt: referenceAt,
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		return fmt.Errorf("put tesouro ipca 10a: %w", err)
	}
	return nil
}

func (db *DB) GetFocusIPCA12m(ctx context.Context) (value *float64, referenceAt, fetchedAt *time.Time, found bool, err error) {
	row, err := db.q.GetMacroRate(ctx, focusIPCA12mRateID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, nil, false, fmt.Errorf("get focus ipca 12m: %w", err)
	}
	return &row.Value, &row.ReferenceAt, &row.FetchedAt, true, nil
}

func (db *DB) PutFocusIPCA12m(ctx context.Context, value float64, referenceAt, fetchedAt time.Time) error {
	err := db.q.UpsertMacroRate(ctx, gen.UpsertMacroRateParams{
		ID:          focusIPCA12mRateID,
		Value:       value,
		ReferenceAt: referenceAt,
		FetchedAt:   fetchedAt,
	})
	if err != nil {
		return fmt.Errorf("put focus ipca 12m: %w", err)
	}
	return nil
}
