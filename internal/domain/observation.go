package domain

import "time"

// Observation é a unidade atômica de dado da ferramenta: não "os dados da
// PETR4", mas um valor de uma métrica, para um período, com de onde veio e
// quando.
//
// Value é *float64 de propósito. Um float64 nu não consegue distinguir "a
// fonte informou ausência" de "a fonte informou zero" — e essa é exatamente a
// confusão que produz médias setoriais corrompidas e semáforo mentiroso.
// Zero e ausente são estados diferentes desde a struct, não a partir da tela.
//
// Os dois carimbos de tempo também não são preciosismo: FetchedAt é quando nós
// coletamos, ReferenceAt é a que data o dado se refere. Sem os dois, a
// ferramenta exibiria "coletado hoje" para um balanço de nove meses atrás.
type Observation struct {
	Ticker      string
	Metric      MetricID
	PeriodKind  string
	PeriodEnd   time.Time
	Value       *float64
	Unit        Unit
	Source      string
	ReferenceAt *time.Time
	FetchedAt   time.Time
	RunID       int64
}

// MetricSet é o conjunto de indicadores conhecidos de um ativo.
//
// Junto com Observation.Value, dá as três distinções que a interface precisa
// sem inventar um enum de estado:
//
//	chave ausente no mapa   → nunca coletado
//	presente, Value == nil  → a fonte informou ausência
//	presente, Value != nil  → valor real, inclusive um zero legítimo
type MetricSet map[MetricID]Observation
