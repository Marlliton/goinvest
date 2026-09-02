// Package domain contém os tipos que atravessam toda a ferramenta: ativos,
// métricas e observações com proveniência.
//
// Este pacote não importa nada de infraestrutura — nem banco, nem rede, nem
// biblioteca de terceiro. É o que permite testar semáforo, alertas e derivados
// sem subir SQLite nem tocar a rede. A regra não é uma convenção de revisão:
// internal/architecture_test/boundaries_test.go a verifica mecanicamente a
// cada `go test ./...`.
package domain

import "time"

// AssetClass é a classe do ativo. A v1 cobre exatamente duas — ETFs e BDRs
// ficaram para a v2. O tipo restringe o que o schema deliberadamente não
// restringe: não há CHECK na coluna correspondente, a garantia vive aqui.
type AssetClass string

const (
	ClassAcao AssetClass = "ACAO"
	ClassFII  AssetClass = "FII"
)

// Asset é a identidade de um papel negociado — o que o usuário digita e o que
// a ferramenta mostra no cabeçalho. Indicadores não moram aqui: eles são
// Observation, porque um número sem proveniência não é utilizável.
type Asset struct {
	Ticker    string
	Class     AssetClass
	Name      string
	UpdatedAt time.Time
}
