package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
)

const focusTTL = 24 * time.Hour

// Filtro composto (Suavizada/baseCalculo) no $filter da Olinda devolve 400: schema declara
// baseCalculo como Edm.Boolean, mas o JSON traz inteiro. Filtra-se no Go depois de desserializar.
const focusPath = "/olinda/servico/Expectativas/versao/v1/odata/ExpectativasMercadoInflacao12Meses?$filter=Indicador%20eq%20'IPCA'&$orderby=Data%20desc&$top=10&$format=json"

type FocusProvider struct {
	client  *fetch.Client
	baseURL string
	now     func() time.Time
}

func NewFocusProvider(client *fetch.Client, baseURL string, now func() time.Time) *FocusProvider {
	if now == nil {
		now = time.Now
	}
	return &FocusProvider{client: client, baseURL: strings.TrimSuffix(baseURL, "/"), now: now}
}

func (p *FocusProvider) Name() string { return "bcb-focus" }

type focusPoint struct {
	Data        string  `json:"Data"`
	Mediana     float64 `json:"Mediana"`
	Suavizada   string  `json:"Suavizada"`
	BaseCalculo int     `json:"baseCalculo"`
}

type focusResponse struct {
	Value []focusPoint `json:"value"`
}

func (p *FocusProvider) FocusIPCA12m(ctx context.Context, force bool) (rate float64, referenceAt time.Time, err error) {
	url := p.baseURL + focusPath

	body, err := p.client.GetRaw(ctx, url, "bcb_focus_ipca12m", focusTTL, force)
	if err != nil {
		return 0, time.Time{}, err
	}

	var resp focusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: focus: %w", err)
	}

	var best *focusPoint
	var bestDate time.Time
	for i := range resp.Value {
		point := resp.Value[i]
		if point.Suavizada != "S" || point.BaseCalculo != 0 {
			continue
		}
		date, err := time.Parse("2006-01-02", point.Data)
		if err != nil {
			return 0, time.Time{}, fmt.Errorf("bcb: focus: data %q inválida: %w", point.Data, err)
		}
		if best == nil || date.After(bestDate) {
			best = &point
			bestDate = date
		}
	}
	if best == nil {
		return 0, time.Time{}, fmt.Errorf("bcb: focus: nenhum ponto suavizado/base-calculo=0 na resposta")
	}

	return best.Mediana / 100, bestDate, nil
}
