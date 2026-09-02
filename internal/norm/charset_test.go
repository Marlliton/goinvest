package norm_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/marlliton/goinvest/internal/norm"
)

func TestDecodeISO88591(t *testing.T) {
	// Bytes latin-1 crus: 0xE1=á 0xC7=Ç 0xD5=Õ 0xDA=Ú
	raw := []byte("M\xe1quinas e Equipamentos \xc7\xd5 PEN\xdaLTIMO")

	r, err := norm.DecodeISO88591(bytes.NewReader(raw))
	require.NoError(t, err)

	decoded, err := io.ReadAll(r)
	require.NoError(t, err)

	require.Equal(t, "Máquinas e Equipamentos ÇÕ PENÚLTIMO", string(decoded))
}
