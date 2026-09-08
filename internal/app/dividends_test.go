package app_test

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/app"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

func seedDividends(t *testing.T, db *store.DB, ticker string, events ...domain.DividendEvent) {
	t.Helper()
	for i := range events {
		events[i].Ticker = ticker
	}
	require.NoError(t, db.InsertDividendEvents(t.Context(), events))
}

func exOn(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func event(exDate time.Time, kind domain.DividendType, raw, factor float64) domain.DividendEvent {
	return domain.DividendEvent{
		ExDate:           exDate,
		Type:             kind,
		TypeRaw:          string(kind),
		ValuePerShareRaw: raw,
		SharesFactor:     factor,
		Source:           "fundamentus:proventos",
		FetchedAt:        collectedAt,
	}
}

// BBAS3 publica o provento por lote de mil ações: sem a divisão, a soma anual
// sai mil vezes inflada.
func TestDividends_SharesFactor1000(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, wege3Values())
	seedDividends(t, db, "BBAS3",
		event(exOn(2024, time.March, 14), domain.DividendJCP, 0.4398, 1000))

	view, err := app.Dividends(t.Context(), db, "BBAS3")
	require.NoError(t, err)
	require.Len(t, view.Lines, 1)
	require.InDelta(t, 0.0004398, view.Lines[0].ValuePerShare, 1e-12)
}

func TestDividends_TypesSeparated(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, wege3Values())
	seedDividends(t, db, "BBAS3",
		event(exOn(2024, time.March, 14), domain.DividendJCP, 0.20, 1),
		event(exOn(2024, time.February, 20), domain.DividendCash, 0.35, 1))

	view, err := app.Dividends(t.Context(), db, "BBAS3")
	require.NoError(t, err)
	require.Len(t, view.Lines, 2)
	require.Equal(t, domain.DividendJCP, view.Lines[0].Type, "a data-com mais recente vem primeiro")
	require.Equal(t, domain.DividendCash, view.Lines[1].Type)
}

func TestDividends_NoEvents(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, wege3Values())

	_, err := app.Dividends(t.Context(), db, "BBAS3")
	require.ErrorIs(t, err, app.ErrNoDividends)
}

func TestDividends_UnknownTicker(t *testing.T) {
	db := openTemp(t)

	_, err := app.Dividends(t.Context(), db, "BBAS3")
	require.ErrorIs(t, err, app.ErrNoData)
}

func TestDividends_ResolvesFractionalTicker(t *testing.T) {
	db := openTemp(t)
	seed(t, db, "BBAS3", domain.ClassStock, wege3Values())
	seedDividends(t, db, "BBAS3",
		event(exOn(2024, time.March, 14), domain.DividendJCP, 0.20, 1))

	view, err := app.Dividends(t.Context(), db, "BBAS3F")
	require.NoError(t, err)
	require.Equal(t, "BBAS3", view.Ticker)
	require.Len(t, view.Lines, 1)
}
