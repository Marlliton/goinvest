// Package provider declara o que uma fonte de dados sabe fazer. As interfaces
// são pequenas e por capacidade: uma fonte implementa só o que ela entrega, em
// vez de preencher métodos vazios de uma interface única.
package provider

import (
	"context"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
)

type Namer interface {
	Name() string
}

// UniverseProvider é a fonte que devolve o mercado inteiro de uma classe numa
// requisição. force ignora o TTL de cache do fetch.
type UniverseProvider interface {
	Namer
	SourceID(class domain.AssetClass) string
	Universe(ctx context.Context, class domain.AssetClass, force bool) ([]domain.Observation, error)
}

// IdentityProvider é a fonte de cadastro: quem é o ativo e em que setor ele
// está, não quanto ele vale.
type IdentityProvider interface {
	Namer
	Companies(ctx context.Context, force bool) ([]identity.CompanyRef, error)
	Detail(ctx context.Context, codeCVM string, force bool) (identity.CompanyDetail, error)
}

// FIIISINProvider existe separada porque a B3 não cobre fundos: a identidade de
// FII vem do informe da CVM, por outro caminho.
type FIIISINProvider interface {
	Namer
	ISINByCNPJ(ctx context.Context, force bool) (map[string]string, error)
}
