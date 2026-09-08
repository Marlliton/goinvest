package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func proventosCmd(t *testing.T, deps rootDeps, out *bytes.Buffer, args ...string) error {
	t.Helper()

	cmd := newProventosCmd(deps)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(t.Context())
}

func TestProventosCmd_NoDividendsPointsToDetalhar(t *testing.T) {
	var out bytes.Buffer

	err := proventosCmd(t, testDeps(t), &out, "wege3")

	require.Error(t, err)
	require.Contains(t, err.Error(), "goinvest detalhar WEGE3")
	require.NotContains(t, err.Error(), "goinvest sync")
}

func TestProventosCmd_UnknownTickerPointsToSync(t *testing.T) {
	var out bytes.Buffer

	err := proventosCmd(t, testDeps(t), &out, "PETR4")

	require.Error(t, err)
	require.Contains(t, err.Error(), "goinvest sync")
	require.NotContains(t, err.Error(), "goinvest detalhar")
}

func TestProventosCmd_ListsSeriesPerShare(t *testing.T) {
	var out bytes.Buffer
	deps := testDeps(t)

	require.NoError(t, deps.DB.InsertDividendEvents(t.Context(), []domain.DividendEvent{{
		Ticker:           "WEGE3",
		ExDate:           time.Date(2024, time.March, 14, 0, 0, 0, 0, time.UTC),
		Type:             domain.DividendJCP,
		TypeRaw:          "JRS CAP PRÓPRIO",
		ValuePerShareRaw: 0.4398,
		SharesFactor:     1000,
		Source:           "fundamentus:proventos",
		FetchedAt:        collectedAt,
	}}))

	require.NoError(t, proventosCmd(t, deps, &out, "wege3"))

	text := out.String()
	require.Contains(t, text, "WEGE3 · proventos")
	require.Contains(t, text, "14/03/2024 · JCP · R$ 0,0004398")
	require.Contains(t, text, "2024: R$ 0,0004398")
}
