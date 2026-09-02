// Package norm normaliza texto de fontes externas na fronteira de entrada:
// encoding, formato numérico e códigos de ausência disfarçados de valor.
package norm

import (
	"strconv"
	"strings"
)

// O segundo par de espaços é o não separável (U+00A0), comum no HTML de origem.
var textCleaner = strings.NewReplacer(
	"%", "",
	"R$", "",
	" ", "",
	" ", "",
)

// ParseBRNumber lê um número no formato pt-BR: ponto como separador de milhar,
// vírgula como decimal. O bool separa ausência de zero: ausência e texto
// corrompido devolvem (0, false), nunca (0, true).
//
// Só serve para pt-BR. O CSV da CVM usa a convenção oposta e precisa do seu
// próprio parser.
func ParseBRNumber(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	switch s {
	case "", "-", "N/A", "n/a":
		return 0, false
	}

	s = textCleaner.Replace(s)
	s = strings.ReplaceAll(s, ".", "")
	s = strings.ReplaceAll(s, ",", ".")

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// ParseBRPercent devolve a fração: "33,1%" vira 0.331, não 33.1.
//
// Um zero explícito volta como (0, true). Decidir que aquele zero significa
// "não sei" depende da métrica e é trabalho de IsAbsenceSentinel.
func ParseBRPercent(s string) (float64, bool) {
	v, ok := ParseBRNumber(s)
	if !ok {
		return 0, false
	}
	return v / 100, true
}
