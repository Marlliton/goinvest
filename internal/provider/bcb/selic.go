// Package bcb lê séries do SGS do Banco Central.
package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
)

const selicTTL = 24 * time.Hour

const selicPath = "/dados/serie/bcdata.sgs.432/dados/ultimos/5?formato=json"

type Provider struct {
	client  *fetch.Client
	baseURL string
	now     func() time.Time
}

func NewProvider(client *fetch.Client, baseURL string, now func() time.Time) *Provider {
	if now == nil {
		now = time.Now
	}
	return &Provider{client: client, baseURL: strings.TrimSuffix(baseURL, "/"), now: now}
}

func (p *Provider) Name() string { return "bcb" }

type sgsPoint struct {
	Data  string `json:"data"`
	Valor string `json:"valor"`
}

func (p *Provider) Selic(ctx context.Context, force bool) (rate float64, referenceAt time.Time, err error) {
	url := p.baseURL + selicPath

	// GetRaw, nunca Get: o SGS já serve UTF-8, e decodificar como ISO-8859-1
	// corromperia o corpo.
	body, err := p.client.GetRaw(ctx, url, "bcb_selic", selicTTL, force)
	if err != nil {
		return 0, time.Time{}, err
	}

	var points []sgsPoint
	if err := json.Unmarshal(body, &points); err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: selic: %w", err)
	}
	if len(points) == 0 {
		return 0, time.Time{}, fmt.Errorf("bcb: selic: série 432 devolveu zero pontos")
	}

	last := points[len(points)-1]

	value, err := strconv.ParseFloat(last.Valor, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: selic: valor %q não é numérico: %w", last.Valor, err)
	}
	// O SGS publica "14.00" para 14% a.a.; percentual é fração no resto do pipeline.
	value /= 100

	ref, err := time.Parse("02/01/2006", last.Data)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: selic: data %q inválida: %w", last.Data, err)
	}

	return value, ref, nil
}
