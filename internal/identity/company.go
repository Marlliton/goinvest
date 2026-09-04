package identity

// O ticker não vem no cadastro: é resolvido pelo casamento com a raiz
// alfabética.
type CompanyRef struct {
	Ticker         string
	IssuingCompany string
	CodeCVM        string
	CNPJ           string
}

type CompanyCode struct {
	Code string
	ISIN string
}

type CompanyDetail struct {
	Code                   string
	CodeCVM                string
	CNPJ                   string
	IndustryClassification string
	OtherCodes             []CompanyCode
}
