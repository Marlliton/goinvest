package domain

import "time"

type Observation struct {
	Ticker      string
	Metric      MetricID
	PeriodKind  string
	PeriodEnd   time.Time
	Value       *float64 // nil = a fonte informou ausência; ponteiro para 0 = zero legítimo
	Unit        Unit
	Source      string
	ReferenceAt *time.Time // nil quando a fonte não informa competência
	FetchedAt   time.Time
	RunID       int64
}

// MetricSet distingue "nunca coletado" (chave ausente no mapa) de "coletado
// sem valor" (Observation.Value nil).
type MetricSet map[MetricID]Observation
