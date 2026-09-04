// Package catalog é o cardápio de métricas: quais existem, em que bloco
// aparecem, para qual classe se aplicam e como são derivadas. Não faz I/O.
package catalog

import (
	"bytes"
	_ "embed"
	"fmt"
	"slices"

	"github.com/marlliton/goinvest/internal/domain"
	"gopkg.in/yaml.v3"
)

//go:embed metrics.yaml
var metricsYAML []byte

//go:embed glossary.yaml
var glossaryYAML []byte

type Block struct {
	ID    string
	Label string
	Order int
}

type Metric struct {
	ID      domain.MetricID
	Label   string
	Block   string
	Order   int
	Unit    domain.Unit
	Classes []domain.AssetClass
	Derived bool
	Formula string
	Inputs  []domain.MetricID
	// Em que situação o número é calculável mas a pergunta que ele responde
	// não faz sentido. Obrigatório em derivado: é o campo que força a decisão
	// a ser tomada quando a métrica nasce, não quando o usuário se confunde.
	NotApplicable string
	Percentile    bool
	// Damodaran: múltiplo com denominador negativo sai da distribuição em vez
	// de virar cauda, senão a mediana do setor desloca sem significado.
	ExcludeNegative bool
	// Segmentos em que a fonte publica 0,00 no lugar de "não se aplica".
	SentinelSegments []string
}

type Catalog struct {
	Blocks   []Block
	Metrics  []Metric
	Glossary map[domain.MetricID]string
}

func Load() (*Catalog, error) {
	return loadFrom(metricsYAML, glossaryYAML)
}

// MetricsFor devolve as métricas aplicáveis à classe, já na ordem de exibição
// (bloco, depois métrica dentro do bloco).
func (c *Catalog) MetricsFor(class domain.AssetClass) []Metric {
	out := make([]Metric, 0, len(c.Metrics))
	for _, m := range c.Metrics {
		if slices.Contains(m.Classes, class) {
			out = append(out, m)
		}
	}
	return out
}

// BlocksOrdered devolve os blocos por Order crescente. A garantia é do método,
// não da ordem de declaração no YAML.
func (c *Catalog) BlocksOrdered() []Block {
	out := slices.Clone(c.Blocks)
	slices.SortFunc(out, func(a, b Block) int { return a.Order - b.Order })
	return out
}

type rawFile struct {
	Blocks  map[string]rawBlock `yaml:"blocks"`
	Metrics []rawMetric         `yaml:"metrics"`
}

type rawBlock struct {
	Label string `yaml:"label"`
	Order int    `yaml:"order"`
}

type rawMetric struct {
	ID               string   `yaml:"id"`
	Label            string   `yaml:"label"`
	Block            string   `yaml:"block"`
	Order            int      `yaml:"order"`
	Unit             string   `yaml:"unit"`
	Classes          []string `yaml:"classes"`
	Derived          bool     `yaml:"derived"`
	Formula          string   `yaml:"formula"`
	Inputs           []string `yaml:"inputs"`
	NotApplicable    string   `yaml:"not_applicable"`
	Percentile       bool     `yaml:"percentile"`
	ExcludeNegative  bool     `yaml:"distribution_excludes_negative"`
	SentinelSegments []string `yaml:"sentinel_segments"`
}

func loadFrom(metricsData, glossaryData []byte) (*Catalog, error) {
	var raw rawFile
	if err := decodeStrict(metricsData, &raw); err != nil {
		return nil, fmt.Errorf("metrics: %w", err)
	}

	var glossary map[domain.MetricID]string
	if err := decodeStrict(glossaryData, &glossary); err != nil {
		return nil, fmt.Errorf("glossary: %w", err)
	}

	blocks := make([]Block, 0, len(raw.Blocks))
	for id, b := range raw.Blocks {
		blocks = append(blocks, Block{ID: id, Label: b.Label, Order: b.Order})
	}
	slices.SortFunc(blocks, func(a, b Block) int { return a.Order - b.Order })

	metrics := make([]Metric, 0, len(raw.Metrics))
	seen := make(map[domain.MetricID]struct{}, len(raw.Metrics))
	for _, rm := range raw.Metrics {
		m, err := rm.toMetric()
		if err != nil {
			return nil, err
		}
		if _, dup := seen[m.ID]; dup {
			return nil, fmt.Errorf("metric %q declared twice", m.ID)
		}
		seen[m.ID] = struct{}{}

		if _, ok := raw.Blocks[m.Block]; !ok {
			return nil, fmt.Errorf("metric %q references unknown block %q", m.ID, m.Block)
		}
		if _, ok := glossary[m.ID]; !ok {
			return nil, fmt.Errorf("metric %q has no glossary entry", m.ID)
		}
		metrics = append(metrics, m)
	}

	for _, m := range metrics {
		for _, in := range m.Inputs {
			if _, ok := seen[in]; !ok {
				return nil, fmt.Errorf("metric %q lists unknown input %q", m.ID, in)
			}
		}
	}

	blockOrder := make(map[string]int, len(blocks))
	for _, b := range blocks {
		blockOrder[b.ID] = b.Order
	}
	slices.SortFunc(metrics, func(a, b Metric) int {
		if d := blockOrder[a.Block] - blockOrder[b.Block]; d != 0 {
			return d
		}
		return a.Order - b.Order
	})

	return &Catalog{Blocks: blocks, Metrics: metrics, Glossary: glossary}, nil
}

func (rm rawMetric) toMetric() (Metric, error) {
	if rm.ID == "" {
		return Metric{}, fmt.Errorf("metric without id")
	}
	id := domain.MetricID(rm.ID)

	unit, err := parseUnit(rm.Unit)
	if err != nil {
		return Metric{}, fmt.Errorf("metric %q: %w", id, err)
	}
	if len(rm.Classes) == 0 {
		return Metric{}, fmt.Errorf("metric %q applies to no class", id)
	}
	classes := make([]domain.AssetClass, 0, len(rm.Classes))
	for _, c := range rm.Classes {
		class, err := parseClass(c)
		if err != nil {
			return Metric{}, fmt.Errorf("metric %q: %w", id, err)
		}
		classes = append(classes, class)
	}

	switch {
	case rm.Derived && (rm.Formula == "" || len(rm.Inputs) == 0):
		return Metric{}, fmt.Errorf("metric %q is derived but has no formula or inputs", id)
	case rm.Derived && rm.NotApplicable == "":
		return Metric{}, fmt.Errorf("metric %q is derived but does not declare when it does not apply", id)
	case !rm.Derived && rm.Formula != "":
		return Metric{}, fmt.Errorf("metric %q has a formula but is not derived", id)
	}

	inputs := make([]domain.MetricID, 0, len(rm.Inputs))
	for _, in := range rm.Inputs {
		inputs = append(inputs, domain.MetricID(in))
	}

	return Metric{
		ID:               id,
		Label:            rm.Label,
		Block:            rm.Block,
		Order:            rm.Order,
		Unit:             unit,
		Classes:          classes,
		Derived:          rm.Derived,
		Formula:          rm.Formula,
		Inputs:           inputs,
		NotApplicable:    rm.NotApplicable,
		Percentile:       rm.Percentile,
		ExcludeNegative:  rm.ExcludeNegative,
		SentinelSegments: rm.SentinelSegments,
	}, nil
}

// O YAML usa o vocabulário curto e minúsculo; o valor persistido do domínio é
// o do mercado brasileiro em caixa alta. A tradução mora aqui, e classe
// desconhecida é erro de carga, não métrica que some em silêncio.
func parseClass(s string) (domain.AssetClass, error) {
	switch s {
	case "acao":
		return domain.ClassStock, nil
	case "fii":
		return domain.ClassFII, nil
	}
	return "", fmt.Errorf("unknown asset class %q", s)
}

func parseUnit(s string) (domain.Unit, error) {
	u := domain.Unit(s)
	switch u {
	case domain.UnitBRL, domain.UnitRatio, domain.UnitPercent, domain.UnitCount:
		return u, nil
	}
	return "", fmt.Errorf("unknown unit %q", s)
}

// KnownFields transforma um campo com nome errado em erro de carga; sem ele o
// yaml.v3 ignora a chave desconhecida e o valor vira o zero do tipo.
func decodeStrict(data []byte, out any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	return dec.Decode(out)
}
