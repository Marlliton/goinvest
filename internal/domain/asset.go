// Package domain contém os tipos que atravessam toda a ferramenta e não
// importa nenhum pacote de infraestrutura.
package domain

import "time"

type AssetClass string

const (
	ClassStock AssetClass = "ACAO"
	ClassFII   AssetClass = "FII"
)

type Asset struct {
	Ticker    string
	Class     AssetClass
	Name      string
	UpdatedAt time.Time
}
