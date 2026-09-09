package config_test

import (
	"path/filepath"
	"testing"

	"github.com/marlliton/goinvest/internal/config"
	"github.com/stretchr/testify/require"
)

func fixture(name string) string {
	return filepath.Join("testdata", name)
}

func TestLoadAbsentFileReturnsDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nao-existe.toml")

	result, err := config.Load(path)

	require.NoError(t, err)
	require.False(t, result.FromFile)
	require.Equal(t, path, result.Path)
	require.Equal(t, config.Default(), result.Tributacao)
}

func TestLoadValidFile(t *testing.T) {
	result, err := config.Load(fixture("valid.toml"))

	require.NoError(t, err)
	require.True(t, result.FromFile)
	require.Equal(t, 0.0, result.Tributacao.AliquotaAtivo)
	require.Equal(t, 0.15, result.Tributacao.AliquotaRendaFixa)
}

func TestLoadUnknownFieldFails(t *testing.T) {
	_, err := config.Load(fixture("unknown_field.toml"))

	require.ErrorContains(t, err, "aliquota_fii")
}

func TestLoadOutOfRangeFails(t *testing.T) {
	_, err := config.Load(fixture("out_of_range.toml"))

	require.ErrorContains(t, err, "aliquota_renda_fixa")
	require.ErrorContains(t, err, "2.5")
}

func TestLoadMalformedTOMLFails(t *testing.T) {
	_, err := config.Load(fixture("malformed.toml"))

	require.Error(t, err)
}

func TestLoadIsStatelessAcrossCalls(t *testing.T) {
	first, err := config.Load(fixture("valid.toml"))
	require.NoError(t, err)

	second, err := config.Load(fixture("valid.toml"))
	require.NoError(t, err)

	require.Equal(t, first, second)
}

func TestDefaultNeverChanges(t *testing.T) {
	require.Equal(t, config.Default(), config.Default())
}
