package norm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

// Cada linha desta tabela é uma classe de bug histórica distinta documentada na
// Armadilha 9 — erro de 1000x (milhar tratado como decimal), erro de 100x
// (percentual bruto vs fração), e ausência virando zero. Não é uma amostra: é o
// conjunto completo de formatos observados nos payloads reais.
func TestParseNumeroBR(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		valor   float64
		ok      bool
	}{
		{"decimal simples", "49,70", 49.70, true},
		{"milhar com ponto sem decimal", "208.607.000.000", 208607000000, true},
		{"milhar com ponto e decimal", "1.012.240.000,00", 1012240000.00, true},
		{"negativo", "-0,20", -0.20, true},
		{"traco e ausencia, nao zero", "-", 0, false},
		{"string vazia e ausencia", "", 0, false},
		{"N/A e ausencia", "N/A", 0, false},
		{"erro de parse nunca vira zero valido", "12,3,4", 0, false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			valor, ok := norm.ParseNumeroBR(c.entrada)
			require.Equal(t, c.ok, ok, "flag de presença para %q", c.entrada)
			require.InDelta(t, c.valor, valor, 1e-9, "valor para %q", c.entrada)
		})
	}
}

// ParsePercentualBR devolve a FRAÇÃO, nunca o valor bruto. A ambiguidade entre
// 0.331 e 33.1 é o bug de 100x que a Armadilha 9 diz sobreviver meses.
func TestParsePercentualBR(t *testing.T) {
	casos := []struct {
		nome    string
		entrada string
		valor   float64
		ok      bool
	}{
		{"percentual positivo vira fracao", "33,1%", 0.331, true},
		{"percentual negativo grande", "-362,66%", -3.6266, true},
		{"zero explicito e valido no parser generico", "0,0%", 0, true},
		{"ausencia continua sendo ausencia", "-", 0, false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			valor, ok := norm.ParsePercentualBR(c.entrada)
			require.Equal(t, c.ok, ok, "flag de presença para %q", c.entrada)
			require.InDelta(t, c.valor, valor, 1e-9, "valor para %q", c.entrada)
		})
	}
}
