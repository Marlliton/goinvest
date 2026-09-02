package norm_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

// Armadilha 2: bancos vêm com EV/EBITDA "0,00" no bulk, não com "-". O parser
// genérico está certo em devolver (0, true) — é a validação por métrica que
// sabe que aquele zero é um código de "não sei".
func TestValidate_SentinelaEVEBITDA(t *testing.T) {
	require.True(t, norm.IsSentinelaAusencia("ev_ebitda", 0),
		"EV/EBITDA igual a 0 é sentinela de ausência (caso ITUB4), não valor real")

	require.False(t, norm.IsSentinelaAusencia("ev_ebitda", 4.2),
		"EV/EBITDA com valor real não é sentinela")

	require.False(t, norm.IsSentinelaAusencia("pl", 0),
		"zero em P/L não é o sentinela documentado nesta fase")
}
