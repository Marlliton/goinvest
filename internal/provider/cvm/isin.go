// Package cvm lê o informe mensal de FIIs da CVM. Aqui ele serve só como
// fonte de ISIN, que é o que liga um fundo ao seu ticker.
package cvm

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/norm"
)

// Cadastro de fundo muda por trimestre, na mesma janela do cadastro da B3.
const isinTTL = 30 * 24 * time.Hour

const (
	colCNPJ = "CNPJ_Fundo_Classe"
	colISIN = "Codigo_ISIN"
	colRef  = "Data_Referencia"
)

type Provider struct {
	client  *fetch.Client
	baseURL string
	years   []int
	now     func() time.Time
}

func NewProvider(client *fetch.Client, baseURL string, years []int, now func() time.Time) *Provider {
	if now == nil {
		now = time.Now
	}
	return &Provider{client: client, baseURL: strings.TrimSuffix(baseURL, "/"), years: years, now: now}
}

func (p *Provider) Name() string { return "cvm" }

type record struct {
	isin string
	ref  string
}

// Os anos são combinados porque um fundo grande pode faltar num ano isolado.
func (p *Provider) ISINByCNPJ(ctx context.Context, force bool) (map[string]string, error) {
	best := make(map[string]record)

	for _, year := range p.years {
		url := fmt.Sprintf("%s/inf_mensal_fii_%d.zip", p.baseURL, year)
		// GetRaw, nunca Get: o corpo é um zip, e a decodificação de charset
		// destruiria a estrutura binária antes do unzip.
		body, err := p.client.GetRaw(ctx, url, "cvm_inf_mensal_fii", isinTTL, force)
		if err != nil {
			return nil, err
		}

		byCNPJ, err := parseGeneralCSV(body, year)
		if err != nil {
			return nil, err
		}
		for cnpj, r := range byCNPJ {
			if cur, seen := best[cnpj]; !seen || r.ref > cur.ref {
				best[cnpj] = r
			}
		}
	}

	out := make(map[string]string, len(best))
	for cnpj, r := range best {
		out[cnpj] = r.isin
	}
	return out, nil
}

func parseGeneralCSV(body []byte, year int) (map[string]record, error) {
	z, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return nil, fmt.Errorf("cvm: zip de %d: %w", year, err)
	}

	name := fmt.Sprintf("inf_mensal_fii_geral_%d.csv", year)
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("cvm: abrir %s: %w", name, err)
		}
		defer rc.Close()
		return readGeneral(rc, name)
	}
	return nil, fmt.Errorf("cvm: %s não encontrado no zip", name)
}

func readGeneral(r io.Reader, name string) (map[string]record, error) {
	decoded, err := norm.DecodeISO88591(r)
	if err != nil {
		return nil, fmt.Errorf("cvm: decodificar %s: %w", name, err)
	}

	reader := csv.NewReader(decoded)
	reader.Comma = ';'
	// O arquivo real varia a contagem de campos entre versões do layout, e
	// isso por si só não impede ler as três colunas que importam.
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("cvm: cabeçalho de %s: %w", name, err)
	}
	index, err := columnIndex(header, name)
	if err != nil {
		return nil, err
	}

	out := make(map[string]record)
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("cvm: ler %s: %w", name, err)
		}
		if len(row) <= index[colISIN] {
			continue
		}

		cnpj := digitsOnly(row[index[colCNPJ]])
		isin := strings.TrimSpace(row[index[colISIN]])
		if cnpj == "" || isin == "" {
			continue
		}

		r := record{isin: isin, ref: strings.TrimSpace(row[index[colRef]])}
		if cur, seen := out[cnpj]; !seen || r.ref > cur.ref {
			out[cnpj] = r
		}
	}
	return out, nil
}

func columnIndex(header []string, name string) (map[string]int, error) {
	pos := make(map[string]int, len(header))
	for i, h := range header {
		pos[strings.TrimSpace(h)] = i
	}

	index := make(map[string]int, 3)
	for _, want := range []string{colCNPJ, colISIN, colRef} {
		i, ok := pos[want]
		if !ok {
			return nil, fmt.Errorf("cvm: %s sem a coluna %s", name, want)
		}
		index[want] = i
	}
	return index, nil
}

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
