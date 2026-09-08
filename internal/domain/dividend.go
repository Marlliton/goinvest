package domain

import "time"

type DividendType string

const (
	DividendCash    DividendType = "DIVIDENDO"
	DividendJCP     DividendType = "JCP"
	DividendUnknown DividendType = "DESCONHECIDO"
)

// DividendEvent guarda o valor como a fonte publicou e o fator ao lado, sem
// dividir: um evento antigo cotado em lote de mil ações fica reconhecível como
// tal, e a conversão vira decisão de quem lê.
type DividendEvent struct {
	Ticker           string
	ExDate           time.Time
	PaymentDate      *time.Time // nil quando a fonte não publica a data
	Type             DividendType
	TypeRaw          string
	ValuePerShareRaw float64
	SharesFactor     float64
	Source           string
	FetchedAt        time.Time
}
