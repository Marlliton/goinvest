package domain_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestDividendEventPerShare(t *testing.T) {
	cases := []struct {
		name   string
		raw    float64
		factor float64
		want   float64
		ok     bool
	}{
		{"lote unitário", 0.4398, 1, 0.4398, true},
		{"lote de mil", 0.4398, 1000, 0.0004398, true},
		{"fator zero", 0.4398, 0, 0, false},
		{"fator negativo", 0.4398, -1, 0, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := domain.DividendEvent{ValuePerShareRaw: c.raw, SharesFactor: c.factor}.PerShare()
			require.Equal(t, c.ok, ok)
			require.InDelta(t, c.want, got, 1e-12)
		})
	}
}
