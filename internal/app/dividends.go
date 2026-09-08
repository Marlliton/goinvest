package app

import (
	"context"
	"errors"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
)

var ErrNoDividends = errors.New("nenhum provento coletado. Rode 'goinvest detalhar TICKER' primeiro")

type DividendLine struct {
	ExDate        time.Time
	PaymentDate   *time.Time
	Type          domain.DividendType
	ValuePerShare float64
}

type DividendsView struct {
	Ticker string
	Lines  []DividendLine
}

// PerShare é o ponto único que aplica o fator de lote: dividir de novo aqui
// dividiria duas vezes.
func Dividends(ctx context.Context, db *store.DB, ticker string) (DividendsView, error) {
	asset, found, err := db.GetAsset(ctx, ticker)
	if err != nil {
		return DividendsView{}, err
	}
	if !found {
		return DividendsView{}, ErrNoData
	}

	events, err := db.ListDividendEvents(ctx, asset.AssetID)
	if err != nil {
		return DividendsView{}, err
	}
	if len(events) == 0 {
		return DividendsView{}, ErrNoDividends
	}

	lines := make([]DividendLine, 0, len(events))
	for _, e := range events {
		perShare, ok := e.PerShare()
		if !ok {
			continue
		}
		lines = append(lines, DividendLine{
			ExDate:        e.ExDate,
			PaymentDate:   e.PaymentDate,
			Type:          e.Type,
			ValuePerShare: perShare,
		})
	}
	return DividendsView{Ticker: asset.Ticker, Lines: lines}, nil
}
