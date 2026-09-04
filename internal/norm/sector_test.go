package norm_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/norm"
	"github.com/stretchr/testify/require"
)

func TestCleanSector(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Motores . Compressores e Outros", "Motores. Compressores e Outros"},
		{"Bens Industriais / Máquinas e Equipamentos / Motores . Compressores e Outros",
			"Bens Industriais / Máquinas e Equipamentos / Motores. Compressores e Outros"},
		{"Financeiro / Intermediários Financeiros / Bancos",
			"Financeiro / Intermediários Financeiros / Bancos"},
		{"J. Macêdo", "J. Macêdo"},
		{"Alimentos  Processados", "Alimentos Processados"},
		{"  Bancos  ", "Bancos"},
		{"", ""},
	}
	for _, c := range cases {
		require.Equal(t, c.want, norm.CleanSector(c.in), c.in)
	}
}
