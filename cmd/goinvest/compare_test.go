package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func compareCmd(t *testing.T, deps rootDeps, out *bytes.Buffer, args ...string) error {
	t.Helper()

	cmd := newCompareCmd(deps)
	cmd.SetOut(out)
	cmd.SetErr(out)
	cmd.SetArgs(args)
	return cmd.ExecuteContext(t.Context())
}

func TestCompareCmd_RejectsFewerThanThree(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out, "WEGE3", "ITUB4")
	require.Error(t, err)
}

func TestCompareCmd_RejectsMoreThanEight(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out,
		"AAAA3", "BBBB3", "CCCC3", "DDDD3", "EEEE3", "FFFF3", "GGGG3", "HHHH3", "IIII3")
	require.Error(t, err)
}

// Ticker desconhecido sai no rodapé, não como erro: com oito códigos digitados
// à mão, derrubar tudo por causa de um é o comportamento mais caro.
func TestCompareCmd_UnknownTickerDoesNotFail(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out, "WEGE3", "ITUB4", "PETR4")

	require.NoError(t, err)
	require.Contains(t, out.String(), "PETR4")
	require.Contains(t, out.String(), "goinvest sync")
}

func TestCompareCmd_NormalizesToUpper(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out, "wege3", "itub4", "petr4")

	require.NoError(t, err)
	require.Contains(t, out.String(), "PETR4")
	require.NotContains(t, out.String(), "wege3")
}

func TestCompareCmd_JSONFlag(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out, "WEGE3", "ITUB4", "PETR4", "--json")
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &doc))
	require.Equal(t, float64(1), doc["schema_version"])
}

func TestCompareCmd_TextIsDefault(t *testing.T) {
	var out bytes.Buffer

	err := compareCmd(t, testDeps(t), &out, "WEGE3", "ITUB4", "PETR4")
	require.NoError(t, err)

	require.NotContains(t, out.String(), "schema_version")
	require.Contains(t, out.String(), "Fora da comparação")
}
