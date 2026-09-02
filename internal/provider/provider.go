// Package provider declara o que uma fonte de dados sabe fazer. As interfaces
// são pequenas e por capacidade: uma fonte implementa só o que ela entrega, em
// vez de preencher métodos vazios de uma interface única.
package provider

import (
	"context"

	"github.com/marlliton/goinvest/internal/domain"
)

type Namer interface {
	Name() string
}

// UniverseProvider é a fonte que devolve o mercado inteiro de uma classe numa
// requisição. force ignora o TTL de cache do fetch.
type UniverseProvider interface {
	Namer
	Universe(ctx context.Context, class domain.AssetClass, force bool) ([]domain.Observation, error)
}
