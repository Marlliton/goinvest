package norm

import (
	"io"

	"golang.org/x/net/html/charset"
)

const fundamentusContentType = "text/html; charset=iso-8859-1"

// DecodeISO88591 transcodifica um fluxo ISO-8859-1 para UTF-8. Deve ser
// chamada uma única vez, na entrada: converter latin-1 com string(bytes)
// produz UTF-8 inválido sem devolver erro.
func DecodeISO88591(r io.Reader) (io.Reader, error) {
	return charset.NewReader(r, fundamentusContentType)
}
