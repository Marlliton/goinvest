package fundamentus_test

import (
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func dividends(t *testing.T, ticker string, class domain.AssetClass) []domain.DividendEvent {
	t.Helper()
	events, err := newTickerProvider(t).Dividends(t.Context(), ticker, class, false)
	require.NoError(t, err)
	require.NotEmpty(t, events)
	return events
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("02/01/2006", s)
	require.NoError(t, err)
	return d
}

func TestParseDividends(t *testing.T) {
	cases := map[string]struct {
		wantFactors map[float64]bool
		wantTypes   map[domain.DividendType]bool
		wantCount   int
	}{
		"BBAS3": {
			wantFactors: map[float64]bool{1: true, 1000: true},
			wantTypes:   map[domain.DividendType]bool{domain.DividendJCP: true, domain.DividendCash: true},
			wantCount:   193,
		},
		"ITSA4": {
			wantFactors: map[float64]bool{1: true},
			wantTypes:   map[domain.DividendType]bool{domain.DividendJCP: true, domain.DividendCash: true},
			wantCount:   249,
		},
	}

	for ticker, tc := range cases {
		t.Run(ticker, func(t *testing.T) {
			events := dividends(t, ticker, domain.ClassStock)
			require.Len(t, events, tc.wantCount)

			factors := map[float64]bool{}
			types := map[domain.DividendType]bool{}
			for _, e := range events {
				factors[e.SharesFactor] = true
				types[e.Type] = true
				require.Equal(t, ticker, e.Ticker)
				require.False(t, e.ExDate.IsZero(), "data-com sempre existe na fonte")
				require.NotEmpty(t, e.TypeRaw)
			}
			require.Equal(t, tc.wantFactors, factors)
			require.Equal(t, tc.wantTypes, types)
		})
	}
}

// "JRS CAP PRÓPRIO" e "JRS CAP PROPRIO" convivem na mesma tabela: classificar
// pelo texto cru perderia metade dos eventos de JCP.
func TestParseDividendsClassifiesAccentAndCaseVariants(t *testing.T) {
	byRaw := map[string]domain.DividendType{}
	for _, e := range dividends(t, "BBAS3", domain.ClassStock) {
		byRaw[e.TypeRaw] = e.Type
	}

	require.Equal(t, map[string]domain.DividendType{
		"DIVIDENDO":       domain.DividendCash,
		"Dividendo":       domain.DividendCash,
		"JRS CAP PROPRIO": domain.DividendJCP,
		"JRS CAP PRÓPRIO": domain.DividendJCP,
		"JUROS":           domain.DividendJCP,
		"Juros":           domain.DividendJCP,
	}, byRaw)
}

// O fator é do evento, não do ticker: a mesma tabela mistura lote de mil ações
// com valor por ação.
func TestParseDividendsKeepsRawValueAndFactorSeparate(t *testing.T) {
	var found bool
	for _, e := range dividends(t, "BBAS3", domain.ClassStock) {
		if e.ExDate.Equal(date(t, "14/08/2003")) && e.SharesFactor == 1000 {
			require.InDelta(t, 0.4398, e.ValuePerShareRaw, 1e-9)
			found = true
		}
	}
	require.True(t, found, "evento com fator 1000 não encontrado")
}

// Os pares (data-com, valor) que se repetem em PETR4 são parcelas do mesmo
// provento, não linhas duplicadas.
func TestParseDividends_NoDuplicateDrop(t *testing.T) {
	events := dividends(t, "PETR4", domain.ClassStock)
	require.Len(t, events, 128)

	type comValue struct {
		exDate time.Time
		value  float64
	}
	payments := map[comValue]map[time.Time]bool{}
	for _, e := range events {
		if e.PaymentDate == nil {
			continue
		}
		k := comValue{e.ExDate, e.ValuePerShareRaw}
		if payments[k] == nil {
			payments[k] = map[time.Time]bool{}
		}
		payments[k][*e.PaymentDate] = true
	}

	installments := 0
	for _, dates := range payments {
		if len(dates) > 1 {
			installments++
		}
	}
	require.Equal(t, 5, installments)
}

// A fonte não publica data de pagamento na maior parte dos eventos antigos: é
// ausência de dado histórico, não falha de parse.
func TestParseDividendsAcceptsMissingPaymentDate(t *testing.T) {
	missing := 0
	for _, e := range dividends(t, "BBAS3", domain.ClassStock) {
		if e.PaymentDate == nil {
			missing++
		}
	}
	require.Equal(t, 114, missing)
}

func TestParseFIIDividends(t *testing.T) {
	events := dividends(t, "MXRF11", domain.ClassFII)
	require.Len(t, events, 113)

	for _, e := range events {
		require.Equal(t, 1.0, e.SharesFactor, "a página de FII não tem coluna de fator")
		require.Equal(t, "Rendimento", e.TypeRaw)
		require.Equal(t, domain.DividendUnknown, e.Type)
		require.NotNil(t, e.PaymentDate)
	}

	first := events[0]
	require.Equal(t, date(t, "30/06/2026"), first.ExDate)
	require.Equal(t, date(t, "14/07/2026"), *first.PaymentDate)
	require.InDelta(t, 0.10, first.ValuePerShareRaw, 1e-9)
	require.Equal(t, "fundamentus:fii_proventos", first.Source)
}

// Página errada não pode virar série vazia, que se leria como "ativo sem
// proventos".
func TestParseDividendsFailsOnForeignPage(t *testing.T) {
	_, err := newTickerProvider(t).Dividends(t.Context(), "QUEBRA3", domain.ClassStock, false)
	require.Error(t, err)
}
