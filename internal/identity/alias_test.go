package identity_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/identity"
	"github.com/stretchr/testify/require"
)

func TestFractionalAlias(t *testing.T) {
	require.Equal(t, "PETR4F", identity.FractionalAlias("PETR4"))
	require.Equal(t, "TAEE11F", identity.FractionalAlias("TAEE11"),
		"unit não muda a regra: fracionário é sempre ticker + F")
}
