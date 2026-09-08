package norm

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	unicodenorm "golang.org/x/text/unicode/norm"
)

// A decomposição separa a letra do acento; Mn é a categoria dos acentos
// soltos, e removê-los deixa a letra base.
var accentFolder = transform.Chain(
	unicodenorm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
	unicodenorm.NFC,
)

// FoldUpper devolve a forma comparável de um rótulo de fonte externa: caixa
// alta, sem acento e sem espaço redundante. Serve para classificar, nunca para
// exibir, porque descarta a grafia que a fonte escolheu.
func FoldUpper(s string) string {
	folded, _, err := transform.String(accentFolder, s)
	if err != nil {
		folded = s
	}
	return strings.ToUpper(strings.Join(strings.Fields(folded), " "))
}
