package domain

import (
	"math"
	"time"
)

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

// PerShare aplica o fator de lote uma vez só, para todo mundo que precisa do
// valor por ação. Fator não positivo derruba o evento: a divisão viraria +Inf, e
// um provento nulo lido como infinito contamina soma, média e preço-teto.
func (e DividendEvent) PerShare() (float64, bool) {
	if e.SharesFactor <= 0 {
		return 0, false
	}
	v := e.ValuePerShareRaw / e.SharesFactor
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
