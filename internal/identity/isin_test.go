package identity_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/identity"
	"github.com/stretchr/testify/require"
)

func TestTickerFromISIN(t *testing.T) {
	cases := []struct {
		isin   string
		ticker string
		ok     bool
	}{
		{"BRFVPQCTF015", "FVPQ11", true},
		{"BRMXRFCTF001", "MXRF11", true},
		// Emissor estrangeiro não segue a convenção que a heurística assume.
		{"US0378331005", "", false},
		{"BR", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		ticker, ok := identity.TickerFromISIN(c.isin)
		require.Equal(t, c.ok, ok, c.isin)
		require.Equal(t, c.ticker, ticker, c.isin)
	}
}
