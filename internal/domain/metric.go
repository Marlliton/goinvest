package domain

// MetricID é o identificador estável de um indicador ("pl", "roe", "dy").
// É a chave que liga catálogo, coleta, persistência, derivação e glossário —
// por isso é um tipo próprio, e não uma string solta que qualquer literal
// digitado errado satisfaria.
type MetricID string

// Unit é a unidade de um valor observado. Sem ela, "0,33" é ambíguo entre
// 33% e uma razão de 0,33 — e a formatação na tela erraria por 100×.
type Unit string

const (
	UnitBRL     Unit = "brl"
	UnitRatio   Unit = "ratio"
	UnitPercent Unit = "percent"
	UnitCount   Unit = "count"
)
