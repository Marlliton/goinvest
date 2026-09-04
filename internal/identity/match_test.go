package identity_test

import (
	"testing"

	"github.com/marlliton/goinvest/internal/identity"
	"github.com/stretchr/testify/require"
)

func TestRootOf(t *testing.T) {
	cases := []struct {
		ticker string
		root   string
		ok     bool
	}{
		{"WEGE3", "WEGE", true},
		{"ITUB4", "ITUB", true},
		{"TAEE11", "TAEE", true},
		{"BIDI11", "BIDI", true},
		// Fracionário já virou alias antes de chegar aqui: nunca é candidato a
		// casamento com o cadastro da B3.
		{"PETR4F", "", false},
		{"WEGE", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		root, ok := identity.RootOf(c.ticker)
		require.Equal(t, c.ok, ok, c.ticker)
		require.Equal(t, c.root, root, c.ticker)
	}
}

// A prova de ATIVO-01: o sufixo do ticker não decide nada. TAEE11 é ação e
// MXRF11 é FII, e nenhum dos dois é classificado aqui — quem classifica é a
// fonte que devolveu o código.
func TestClassifyByB3Taxonomy(t *testing.T) {
	companies := []identity.CompanyRef{
		{IssuingCompany: "WEGE", CodeCVM: "5410", CNPJ: "84429695000111"},
		{IssuingCompany: "ITUB", CodeCVM: "19348", CNPJ: "60872504000123"},
		{IssuingCompany: "TAEE", CodeCVM: "20257", CNPJ: "07859971000130"},
	}

	cases := []struct {
		ticker  string
		codeCVM string
		ok      bool
	}{
		{"WEGE3", "5410", true},
		{"ITUB3", "19348", true},
		{"ITUB4", "19348", true},
		// Sufixo 11 igual ao de FII, e mesmo assim casa como ação porque quem
		// responde é o cadastro de companhias abertas.
		{"TAEE11", "20257", true},
		// FII não está no cadastro de companhias abertas: ausência é resultado
		// esperado, não erro.
		{"MXRF11", "", false},
		{"PETR4F", "", false},
	}
	for _, c := range cases {
		codeCVM, ok := identity.MatchByRoot(companies, c.ticker)
		require.Equal(t, c.ok, ok, c.ticker)
		require.Equal(t, c.codeCVM, codeCVM, c.ticker)
	}
}
