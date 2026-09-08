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

// Dividends é leitura pura sobre o que 'goinvest detalhar' já coletou. O fator
// de lote é aplicado por domain.DividendEvent.PerShare, o ponto único do
// sistema: o parser guarda valor e fator separados justamente para que a
// divisão não aconteça duas vezes nem ao contrário.
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
