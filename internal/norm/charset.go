package norm

import (
	"io"

	"golang.org/x/net/html/charset"
)

// contentTypeFundamentus é o Content-Type que as páginas do Fundamentus
// declaram. Serve de palpite para charset.NewReader quando o documento não
// traz BOM nem meta charset próprio.
const contentTypeFundamentus = "text/html; charset=iso-8859-1"

// DecodeISO88591 transcodifica um fluxo ISO-8859-1 para UTF-8.
//
// Decodificar é obrigatório, não opcional: toda a stack brasileira relevante
// serve latin-1, e um string(bytes) direto produz UTF-8 inválido em silêncio —
// "Máquinas" vira "M?quinas", o setor deixa de casar no agrupamento e a tabela
// desalinha porque a largura passa a contar bytes em vez de runas.
//
// Faça isso UMA vez, na fronteira de entrada. Todo o resto do sistema é UTF-8.
func DecodeISO88591(r io.Reader) (io.Reader, error) {
	return charset.NewReader(r, contentTypeFundamentus)
}
