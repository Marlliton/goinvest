// Package norm normaliza texto vindo de fontes de terceiros na fronteira de
// entrada: encoding, formato numérico e códigos de ausência disfarçados de
// valor.
//
// Tudo aqui existe porque nenhum dos erros que este pacote previne lança
// exceção. Um "1.234,56" lido por strconv.ParseFloat vira 1.234 — mil vezes
// menor, sem um único sinal de alerta. É por isso que existe um parser
// canônico, e não strconv espalhado pelo código.
package norm

import (
	"strconv"
	"strings"
)

// limpezaTextual remove os adornos que o Fundamentus mistura ao número.
// O último par é o espaço não separável (U+00A0), abundante em HTML e
// invisível numa inspeção casual do fonte.
var limpezaTextual = strings.NewReplacer(
	"%", "",
	"R$", "",
	" ", "",
	" ", "",
)

// ParseNumeroBR interpreta um número no formato pt-BR do Fundamentus: ponto
// como separador de milhar, vírgula como decimal.
//
// Devolve (valor, true) quando leu um número, e (0, false) quando a fonte
// informou ausência ou o texto está corrompido. Nunca devolve (0, true) para
// ausência — essa é a distinção inteira, e é por isso que a assinatura carrega
// um bool em vez de engolir o erro.
//
// Este parser é deliberadamente só pt-BR. O CSV da CVM usa ponto decimal e dez
// casas — regra oposta — e precisa do seu próprio parser. Confundir os dois é
// fácil e catastrófico.
func ParseNumeroBR(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	switch s {
	case "", "-", "N/A", "n/a":
		return 0, false
	}

	s = limpezaTextual.Replace(s)
	s = strings.ReplaceAll(s, ".", "")  // separador de milhar pt-BR
	s = strings.ReplaceAll(s, ",", ".") // separador decimal pt-BR

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		// Erro de parse é dado corrompido, não valor default. Descartar o erro
		// aqui com `_` seria exatamente a Armadilha 9.
		return 0, false
	}
	return v, true
}

// ParsePercentualBR devolve a FRAÇÃO: "33,1%" vira 0.331, não 33.1.
//
// A escolha está no nome e no comentário de propósito — a ambiguidade entre as
// duas convenções gera um erro de 100x que sobrevive meses porque o número
// continua parecendo plausível.
//
// Um zero explícito ("0,0%") volta como (0, true): é um número que a fonte
// informou. Decidir que aquele zero significa "não sei" depende da métrica, e
// isso é trabalho de IsSentinelaAusencia, não de um parser genérico.
func ParsePercentualBR(s string) (float64, bool) {
	v, ok := ParseNumeroBR(s)
	if !ok {
		return 0, false
	}
	return v / 100, true
}
