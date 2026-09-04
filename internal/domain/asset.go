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
	AssetID   int64
	Ticker    string
	Class     AssetClass
	Name      string
	CNPJ      string
	ISIN      string
	CDCVM     string
	Sector    string
	Subsector string
	Segment   string
	SectorSrc string
	// Ativo morto ou ilíquido fica fora de ranking e de estatística setorial.
	IsActive     bool
	LastLiquidAt *time.Time
	UpdatedAt    time.Time
}
