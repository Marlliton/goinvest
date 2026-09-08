package collect_test

import (
	"context"
	"errors"
	"testing"

	"github.com/marlliton/goinvest/internal/collect"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
	"github.com/stretchr/testify/require"
)

type fakeDetail struct {
	failFor  string
	detailed []string
}

func (*fakeDetail) Name() string { return "fake-detalhes" }

func (f *fakeDetail) Detail(_ context.Context, ticker string, _ domain.AssetClass, _ bool) (domain.MetricSet, error) {
	if ticker == f.failFor {
		return nil, errors.New("fonte fora do ar")
	}
	f.detailed = append(f.detailed, ticker)

	value := 42.0
	return domain.MetricSet{"lucro_liquido": {
		Ticker:     ticker,
		Metric:     "lucro_liquido",
		PeriodKind: "ttm",
		PeriodEnd:  seededAt,
		Value:      &value,
		Unit:       domain.UnitBRL,
		Source:     "fundamentus:detalhes",
		FetchedAt:  seededAt,
	}}, nil
}

func (f *fakeDetail) Dividends(_ context.Context, ticker string, _ domain.AssetClass, _ bool) ([]domain.DividendEvent, error) {
	if ticker == f.failFor {
		return nil, errors.New("fonte fora do ar")
	}
	return []domain.DividendEvent{{
		Ticker:           ticker,
		ExDate:           seededAt,
		Type:             domain.DividendCash,
		TypeRaw:          "DIVIDENDO",
		ValuePerShareRaw: 1.5,
		SharesFactor:     1,
		Source:           "fundamentus:proventos",
		FetchedAt:        seededAt,
	}}, nil
}

func seedAssets(t *testing.T, db *store.DB, tickers ...string) {
	t.Helper()
	for _, ticker := range tickers {
		require.NoError(t, db.UpsertAsset(t.Context(), ticker, domain.ClassStock, "", seededAt))
	}
}

func TestDeep_InvalidTickerDoesNotAbort(t *testing.T) {
	db := openDB(t)
	seedAssets(t, db, "WEGE3", "ITUB4")

	got, err := collect.Deep(t.Context(), collect.DeepConfig{
		DB:      db,
		Detail:  &fakeDetail{},
		Tickers: []string{"WEGE3", "WEG E3", "ITUB4"},
	})
	require.NoError(t, err)
	require.False(t, got.Cancelled)
	require.Len(t, got.Outcomes, 3)

	require.Equal(t, collect.StatusOK, got.Outcomes[0].Status)
	require.Equal(t, collect.StatusPartial, got.Outcomes[1].Status)
	require.Contains(t, got.Outcomes[1].Reason, "corrija")
	require.Equal(t, collect.StatusOK, got.Outcomes[2].Status)
}

func TestDeep_UnsyncedTicker(t *testing.T) {
	db := openDB(t)

	got, err := collect.Deep(t.Context(), collect.DeepConfig{
		DB:      db,
		Detail:  &fakeDetail{},
		Tickers: []string{"PETR4"},
	})
	require.NoError(t, err)
	require.Len(t, got.Outcomes, 1)
	require.Equal(t, collect.StatusPartial, got.Outcomes[0].Status)
	require.Contains(t, got.Outcomes[0].Reason, "goinvest sync")
}

func TestDeep_PartialFailureDoesNotAbort(t *testing.T) {
	db := openDB(t)
	seedAssets(t, db, "WEGE3", "ROMI3", "KEPL3")

	got, err := collect.Deep(t.Context(), collect.DeepConfig{
		DB:      db,
		Detail:  &fakeDetail{failFor: "ROMI3"},
		Tickers: []string{"WEGE3", "ROMI3", "KEPL3"},
	})
	require.NoError(t, err)
	require.Len(t, got.Outcomes, 3)
	require.Equal(t, collect.StatusOK, got.Outcomes[0].Status)
	require.Equal(t, collect.StatusPartial, got.Outcomes[1].Status)
	require.Equal(t, collect.StatusOK, got.Outcomes[2].Status)
}

func TestDeep_PersistsDetailAndDividends(t *testing.T) {
	db := openDB(t)
	seedAssets(t, db, "WEGE3")

	got, err := collect.Deep(t.Context(), collect.DeepConfig{
		DB:      db,
		Detail:  &fakeDetail{},
		Tickers: []string{"WEGE3"},
	})
	require.NoError(t, err)
	require.Equal(t, collect.StatusOK, got.Outcomes[0].Status)

	assetID, found, err := db.AssetIDByTicker(t.Context(), "WEGE3")
	require.NoError(t, err)
	require.True(t, found)

	set, err := db.LatestMetrics(t.Context(), assetID, "WEGE3")
	require.NoError(t, err)
	require.Contains(t, set, domain.MetricID("lucro_liquido"))

	events, err := db.ListDividendEvents(t.Context(), assetID)
	require.NoError(t, err)
	require.Len(t, events, 1)
}

func TestDeep_CancelledStopsWithoutCountingRemainderAsFailure(t *testing.T) {
	db := openDB(t)
	seedAssets(t, db, "WEGE3", "ROMI3", "KEPL3")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	got, err := collect.Deep(ctx, collect.DeepConfig{
		DB:      db,
		Detail:  &fakeDetail{},
		Tickers: []string{"WEGE3", "ROMI3", "KEPL3"},
		OnProgress: func(p collect.Progress) {
			if p.Done == 1 {
				cancel()
			}
		},
	})
	require.NoError(t, err)
	require.True(t, got.Cancelled)
	require.Len(t, got.Outcomes, 1)
}

func TestDeep_UsesCanonicalTickerForFractionalAlias(t *testing.T) {
	db := openDB(t)
	seedAssets(t, db, "PETR4")

	assetID, found, err := db.AssetIDByTicker(t.Context(), "PETR4")
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, db.UpsertAssetAlias(t.Context(), "PETR4F", assetID))

	fake := &fakeDetail{}
	got, err := collect.Deep(t.Context(), collect.DeepConfig{
		DB:      db,
		Detail:  fake,
		Tickers: []string{"PETR4F"},
	})
	require.NoError(t, err)
	require.Equal(t, collect.StatusOK, got.Outcomes[0].Status)
	require.Equal(t, []string{"PETR4"}, fake.detailed, "a fonte não publica página do fracionário")
}
