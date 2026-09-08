package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

type fakeDetail struct {
	asked []string
}

func (*fakeDetail) Name() string { return "fake-detalhes" }

func (f *fakeDetail) Detail(_ context.Context, ticker string, _ domain.AssetClass, _ bool) (domain.MetricSet, error) {
	f.asked = append(f.asked, ticker)

	value := 42.0
	return domain.MetricSet{"lucro_liquido": {
		Ticker:     ticker,
		Metric:     "lucro_liquido",
		PeriodKind: "ttm",
		PeriodEnd:  collectedAt,
		Value:      &value,
		Unit:       domain.UnitBRL,
		Source:     "fundamentus:detalhes",
		FetchedAt:  collectedAt,
	}}, nil
}

func (*fakeDetail) Dividends(_ context.Context, ticker string, _ domain.AssetClass, _ bool) ([]domain.DividendEvent, error) {
	return []domain.DividendEvent{{
		Ticker:           ticker,
		ExDate:           collectedAt,
		Type:             domain.DividendCash,
		TypeRaw:          "DIVIDENDO",
		ValuePerShareRaw: 1.5,
		SharesFactor:     1,
		Source:           "fundamentus:proventos",
		FetchedAt:        collectedAt,
	}}, nil
}

func detalharCmd(t *testing.T, fake *fakeDetail, out *bytes.Buffer, args ...string) error {
	t.Helper()

	deps := testDeps(t)
	deps.Detail = fake

	cmd := newDetalharCmd(deps)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(t.Context())
}

func TestDetalharCmd_RequiresAtLeastOneTicker(t *testing.T) {
	var out bytes.Buffer
	require.Error(t, detalharCmd(t, &fakeDetail{}, &out))
}

func TestDetalharCmd_RejectsMoreThanEightTickers(t *testing.T) {
	var out bytes.Buffer
	err := detalharCmd(t, &fakeDetail{}, &out,
		"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3", "FFFF3", "GGGG3", "HHHH3", "IIII3")
	require.Error(t, err)
}

func TestDetalharCmd_TickerNormalizedUppercase(t *testing.T) {
	var out bytes.Buffer
	fake := &fakeDetail{}

	require.NoError(t, detalharCmd(t, fake, &out, "wege3"))
	require.Equal(t, []string{"WEGE3"}, fake.asked)
	require.Contains(t, out.String(), "✓ WEGE3")
}

func TestDetalharCmd_InvalidTickerDoesNotAbortTheOthers(t *testing.T) {
	var out bytes.Buffer
	fake := &fakeDetail{}

	require.NoError(t, detalharCmd(t, fake, &out, "WEGE3", "WEG E3", "ITUB4"))

	text := out.String()
	require.Contains(t, text, "✓ WEGE3")
	require.Contains(t, text, "✗ WEG E3")
	require.Contains(t, text, "corrija")
	require.Contains(t, text, "✓ ITUB4")
	require.Equal(t, []string{"WEGE3", "ITUB4"}, fake.asked)
}

func TestDetalharCmd_PrintsOneLinePerProgressOutsideTTY(t *testing.T) {
	var out bytes.Buffer

	require.NoError(t, detalharCmd(t, &fakeDetail{}, &out, "WEGE3", "ITUB4"))

	text := out.String()
	require.NotContains(t, text, "\r")
	require.Contains(t, text, "detalhe: 1/2")
	require.Contains(t, text, "detalhe: 2/2")
}
