package tesouro

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/norm"
)

const ipcaTTL = 24 * time.Hour

const dateLayout = "02/01/2006"

// "Tesouro IPCA+" sem cupom, não "com Juros Semestrais": trata-se de um único juro real até
// o vencimento, sem o efeito de reinvestimento de cupom da variante semestral.
const referenceTitleType = "Tesouro IPCA+"

const (
	colTipoTitulo      = "Tipo Titulo"
	colDataVencimento  = "Data Vencimento"
	colDataBase        = "Data Base"
	colTaxaCompraManha = "Taxa Compra Manha"
)

type Provider struct {
	client *fetch.Client
	url    string
	now    func() time.Time
}

func NewProvider(client *fetch.Client, url string, now func() time.Time) *Provider {
	if now == nil {
		now = time.Now
	}
	return &Provider{client: client, url: url, now: now}
}

func (p *Provider) Name() string { return "tesouro-transparente" }

type ipcaRow struct {
	dataVencimento string
	dataBase       time.Time
	taxaCompra     string
}

// Vencimento mais próximo de 10 anos: Damodaran, "Estimating Risk free Rates", recomenda
// casar a duração do título livre de risco com a duração do fluxo de caixa avaliado, e usa
// o título soberano de dez anos como proxy padrão de taxa livre de risco de longo prazo em
// valuation.
func (p *Provider) IPCA10y(ctx context.Context, force bool) (rate float64, referenceAt time.Time, err error) {
	body, err := p.client.GetRaw(ctx, p.url, "tesouro_ipca", ipcaTTL, force)
	if err != nil {
		return 0, time.Time{}, err
	}

	r := csv.NewReader(bytes.NewReader(body))
	r.Comma = ';'
	records, err := r.ReadAll()
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("tesouro: %w", err)
	}
	if len(records) == 0 {
		return 0, time.Time{}, fmt.Errorf("tesouro: csv vazio")
	}

	col, err := headerIndex(records[0])
	if err != nil {
		return 0, time.Time{}, err
	}

	rows, err := filterReferenceType(records[1:], col)
	if err != nil {
		return 0, time.Time{}, err
	}
	if len(rows) == 0 {
		return 0, time.Time{}, fmt.Errorf("tesouro: nenhum título %q na base", referenceTitleType)
	}

	latest := latestDataBase(rows)
	rows = filterByDataBase(rows, latest)

	winner, err := closestToTenYears(rows, p.now())
	if err != nil {
		return 0, time.Time{}, err
	}

	value, ok := norm.ParseBRNumber(winner.taxaCompra)
	if !ok {
		return 0, time.Time{}, fmt.Errorf("tesouro: %s %q não é numérica", colTaxaCompraManha, winner.taxaCompra)
	}

	return value / 100, latest, nil
}

func headerIndex(header []string) (map[string]int, error) {
	col := make(map[string]int, len(header))
	for i, name := range header {
		col[name] = i
	}
	for _, required := range []string{colTipoTitulo, colDataVencimento, colDataBase, colTaxaCompraManha} {
		if _, ok := col[required]; !ok {
			return nil, fmt.Errorf("tesouro: coluna %q ausente no csv", required)
		}
	}
	return col, nil
}

func filterReferenceType(records [][]string, col map[string]int) ([]ipcaRow, error) {
	var rows []ipcaRow
	for _, record := range records {
		if record[col[colTipoTitulo]] != referenceTitleType {
			continue
		}
		dataBase, err := time.Parse(dateLayout, record[col[colDataBase]])
		if err != nil {
			return nil, fmt.Errorf("tesouro: %s %q inválida: %w", colDataBase, record[col[colDataBase]], err)
		}
		rows = append(rows, ipcaRow{
			dataVencimento: record[col[colDataVencimento]],
			dataBase:       dataBase,
			taxaCompra:     record[col[colTaxaCompraManha]],
		})
	}
	return rows, nil
}

func latestDataBase(rows []ipcaRow) time.Time {
	latest := rows[0].dataBase
	for _, row := range rows[1:] {
		if row.dataBase.After(latest) {
			latest = row.dataBase
		}
	}
	return latest
}

func filterByDataBase(rows []ipcaRow, dataBase time.Time) []ipcaRow {
	var filtered []ipcaRow
	for _, row := range rows {
		if row.dataBase.Equal(dataBase) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func closestToTenYears(rows []ipcaRow, now time.Time) (ipcaRow, error) {
	target := now.AddDate(10, 0, 0)

	var winner ipcaRow
	var winnerDiff time.Duration
	for i, row := range rows {
		vencimento, err := time.Parse(dateLayout, row.dataVencimento)
		if err != nil {
			return ipcaRow{}, fmt.Errorf("tesouro: %s %q inválida: %w", colDataVencimento, row.dataVencimento, err)
		}
		diff := time.Duration(math.Abs(float64(vencimento.Sub(target))))
		if i == 0 || diff < winnerDiff {
			winner = row
			winnerDiff = diff
		}
	}
	return winner, nil
}
