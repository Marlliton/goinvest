package domain

import "time"

type DividendType string

const (
	DividendCash    DividendType = "DIVIDENDO"
	DividendJCP     DividendType = "JCP"
	DividendUnknown DividendType = "DESCONHECIDO"
)

// O fator fica ao lado do valor em vez de já aplicado: um evento antigo cotado
// em lote de mil ações continua reconhecível como tal.
type DividendEvent struct {
	Ticker           string
	ExDate           time.Time
	PaymentDate      *time.Time
	Type             DividendType
	TypeRaw          string
	ValuePerShareRaw float64
	SharesFactor     float64
	Source           string
	FetchedAt        time.Time
}
