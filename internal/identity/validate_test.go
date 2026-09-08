package identity_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/identity"
	"github.com/stretchr/testify/require"
)

func TestValidTicker(t *testing.T) {
	cases := []struct {
		ticker string
		want   bool
	}{
		{"WEGE3", true},
		{"TAEE11", true},
		{"MXRF11", true},
		{"PETR4F", true},
		{"TAEE11F", true},
		{"wege3", false},
		{"WEGE", false},
		{"WEGE333", false},
		{"WE", false},
		{"WEG E3", false},
		{"WEGE3'", false},
		{"", false},
	}
	for _, c := range cases {
		require.Equal(t, c.want, identity.ValidTicker(c.ticker), c.ticker)
	}
}
