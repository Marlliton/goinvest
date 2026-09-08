package norm

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	unicodenorm "golang.org/x/text/unicode/norm"
)

// NFD separa a letra do acento e Mn é a categoria do acento solto, então
// removê-los deixa a letra base.
var accentFolder = transform.Chain(
	unicodenorm.NFD,
	runes.Remove(runes.In(unicode.Mn)),
	unicodenorm.NFC,
)

// FoldUpper serve para classificar, nunca para exibir: descarta a grafia que a
// fonte escolheu.
func FoldUpper(s string) string {
	folded, _, err := transform.String(accentFolder, s)
	if err != nil {
		folded = s
	}
	return strings.ToUpper(strings.Join(strings.Fields(folded), " "))
}
