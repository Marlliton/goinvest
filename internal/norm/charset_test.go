package norm_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

// Armadilha 8: toda a stack brasileira relevante serve ISO-8859-1, e Go assume
// UTF-8 em toda parte. Sem decodificação na fronteira, "Máquinas e Equipamentos"
// vira "M?quinas" silenciosamente e o setor deixa de casar no agrupamento.
func TestDecodeISO88591(t *testing.T) {
	// Bytes latin-1 crus: 0xE1 = á, 0xC7 = Ç, 0xD5 = Õ, 0xDA = Ú.
	bruto := []byte("M\xe1quinas e Equipamentos \xc7\xd5 PEN\xdaLTIMO")

	r, err := norm.DecodeISO88591(bytes.NewReader(bruto))
	require.NoError(t, err)

	decodificado, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, "Máquinas e Equipamentos ÇÕ PENÚLTIMO", string(decodificado))
}
