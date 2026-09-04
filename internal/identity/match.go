package identity

import "regexp"

// A raiz é a parte alfabética do ticker, e o sufixo numérico não recebe
// tratamento algum: é isso que impede a classe do ativo de ser inferida do
// código.
var tickerPattern = regexp.MustCompile(`^([A-Z0-9]+?)(\d{1,2})$`)

func RootOf(ticker string) (root string, ok bool) {
	m := tickerPattern.FindStringSubmatch(ticker)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// MatchByRoot devolve ok=false para ticker sem correspondência no cadastro:
// ausência é resultado esperado, não erro.
func MatchByRoot(companies []CompanyRef, ticker string) (codeCVM string, ok bool) {
	root, ok := RootOf(ticker)
	if !ok {
		return "", false
	}
	for _, c := range companies {
		if c.IssuingCompany == root {
			return c.CodeCVM, true
		}
	}
	return "", false
}
