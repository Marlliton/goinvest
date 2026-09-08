package store

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func divEvent(ticker string, exDay int, value float64, payment *time.Time) domain.DividendEvent {
	return domain.DividendEvent{
		Ticker:           ticker,
		ExDate:           time.Date(2026, time.June, exDay, 0, 0, 0, 0, time.UTC),
		PaymentDate:      payment,
		Type:             domain.DividendJCP,
		TypeRaw:          "JRS CAP PRÓPRIO",
		ValuePerShareRaw: value,
		SharesFactor:     1,
		Source:           "fundamentus:proventos",
		FetchedAt:        time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC),
	}
}

func day(d int) *time.Time {
	at := time.Date(2026, time.July, d, 0, 0, 0, 0, time.UTC)
	return &at
}

func seededAssetID(t *testing.T, db *DB, ticker string) int64 {
	t.Helper()
	seedAsset(t, db, ticker, domain.ClassStock)
	id, ok, err := db.AssetIDByTicker(t.Context(), ticker)
	require.NoError(t, err)
	require.True(t, ok)
	return id
}

// O evento sem data de pagamento é o caso que pega: em índice único, NULL não
// colide com NULL.
func TestInsertDividendEvents_Idempotent(t *testing.T) {
	db := openTemp(t)
	assetID := seededAssetID(t, db, "BBAS3")

	events := []domain.DividendEvent{
		divEvent("BBAS3", 1, 0.1025, day(11)),
		divEvent("BBAS3", 2, 0.0345, nil),
	}
	require.NoError(t, db.InsertDividendEvents(t.Context(), events))

	recollected := make([]domain.DividendEvent, len(events))
	for i, e := range events {
		e.FetchedAt = e.FetchedAt.AddDate(0, 0, 30)
		recollected[i] = e
	}
	require.NoError(t, db.InsertDividendEvents(t.Context(), recollected))

	stored, err := db.ListDividendEvents(t.Context(), assetID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
}

func TestInsertDividendEvents_PreservesInstallments(t *testing.T) {
	db := openTemp(t)
	assetID := seededAssetID(t, db, "PETR4")

	require.NoError(t, db.InsertDividendEvents(t.Context(), []domain.DividendEvent{
		divEvent("PETR4", 1, 0.3505, day(20)),
		divEvent("PETR4", 1, 0.3505, day(21)),
	}))

	stored, err := db.ListDividendEvents(t.Context(), assetID)
	require.NoError(t, err)
	require.Len(t, stored, 2)
	require.NotEqual(t, *stored[0].PaymentDate, *stored[1].PaymentDate)
}

func TestListDividendEvents_NeverCollected(t *testing.T) {
	db := openTemp(t)
	assetID := seededAssetID(t, db, "WEGE3")

	stored, err := db.ListDividendEvents(t.Context(), assetID)
	require.NoError(t, err)
	require.Empty(t, stored)
}

func TestListDividendEventsRoundTrip(t *testing.T) {
	db := openTemp(t)
	assetID := seededAssetID(t, db, "BBAS3")

	want := divEvent("BBAS3", 3, 0.4398, day(15))
	want.Type = domain.DividendCash
	want.TypeRaw = "Dividendo"
	want.SharesFactor = 1000
	require.NoError(t, db.InsertDividendEvents(t.Context(), []domain.DividendEvent{
		divEvent("BBAS3", 1, 0.1025, nil),
		want,
	}))

	stored, err := db.ListDividendEvents(t.Context(), assetID)
	require.NoError(t, err)
	require.Len(t, stored, 2)

	require.Equal(t, want.ExDate, stored[0].ExDate.UTC(), "ordem é da data-com mais recente para a mais antiga")
	require.Equal(t, want.Type, stored[0].Type)
	require.Equal(t, want.TypeRaw, stored[0].TypeRaw)
	require.InDelta(t, want.ValuePerShareRaw, stored[0].ValuePerShareRaw, 1e-9)
	require.InDelta(t, want.SharesFactor, stored[0].SharesFactor, 1e-9)
	require.Equal(t, want.Source, stored[0].Source)
	require.Equal(t, *want.PaymentDate, stored[0].PaymentDate.UTC())
	require.Nil(t, stored[1].PaymentDate)
}

func TestInsertDividendEventsRequiresKnownAsset(t *testing.T) {
	db := openTemp(t)

	err := db.InsertDividendEvents(t.Context(), []domain.DividendEvent{divEvent("BBAS3", 1, 0.1025, nil)})
	require.Error(t, err)
}
