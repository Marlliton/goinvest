package identity

import "regexp"

// Formato completo, ao contrário de tickerPattern, que só isola a raiz: aqui a
// entrada é digitada pelo usuário e vira query string, então o dígito a mais
// precisa ser rejeitado antes de virar requisição.
var validTickerPattern = regexp.MustCompile(`^[A-Z]{4}\d{1,2}F?$`)

func ValidTicker(ticker string) bool {
	return validTickerPattern.MatchString(ticker)
}
