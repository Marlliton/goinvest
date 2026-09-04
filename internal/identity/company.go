package identity

// CompanyRef é uma linha do cadastro de companhias abertas. O ticker não vem
// nessa listagem: ele é resolvido pelo casamento com a raiz alfabética.
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

// CompanyDetail é o cadastro de uma companhia: um codeCVM cobre todas as
// classes de ação dela, listadas em OtherCodes.
type CompanyDetail struct {
	Code                   string
	CodeCVM                string
	CNPJ                   string
	IndustryClassification string
	OtherCodes             []CompanyCode
}
