package bcb

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

const cdiTTL = 24 * time.Hour

const cdiPath = "/dados/serie/bcdata.sgs.4389/dados/ultimos/5?formato=json"

func (p *Provider) CDI(ctx context.Context, force bool) (rate float64, referenceAt time.Time, err error) {
	url := p.baseURL + cdiPath

	body, err := p.client.GetRaw(ctx, url, "bcb_cdi", cdiTTL, force)
	if err != nil {
		return 0, time.Time{}, err
	}

	var points []sgsPoint
	if err := json.Unmarshal(body, &points); err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: cdi: %w", err)
	}
	if len(points) == 0 {
		return 0, time.Time{}, fmt.Errorf("bcb: cdi: série 4389 devolveu zero pontos")
	}

	last := points[len(points)-1]

	value, err := strconv.ParseFloat(last.Valor, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: cdi: valor %q não é numérico: %w", last.Valor, err)
	}
	value /= 100

	ref, err := time.Parse("02/01/2006", last.Data)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("bcb: cdi: data %q inválida: %w", last.Data, err)
	}

	return value, ref, nil
}
