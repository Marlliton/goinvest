// Package b3 lê o cadastro de companhias abertas da B3: identidade e
// taxonomia setorial, não cotação.
package b3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/identity"
)

// Cadastro muda por trimestre; um mês de cache ainda pega mudança de setor
// antes de qualquer decisão de investimento.
const registryTTL = 30 * 24 * time.Hour

const pageSize = 120

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

func (p *Provider) Name() string { return "b3" }

type companiesPage struct {
	Page struct {
		PageNumber int `json:"pageNumber"`
		PageSize   int `json:"pageSize"`
		TotalPages int `json:"totalPages"`
	} `json:"page"`
	Results []struct {
		CodeCVM        string `json:"codeCVM"`
		IssuingCompany string `json:"issuingCompany"`
		CNPJ           string `json:"cnpj"`
	} `json:"results"`
}

func (p *Provider) Companies(ctx context.Context, force bool) ([]identity.CompanyRef, error) {
	var out []identity.CompanyRef

	for page, total := 1, 1; page <= total; page++ {
		decoded, err := p.companiesPage(ctx, page, force)
		if err != nil {
			return nil, err
		}
		if decoded.Page.TotalPages < 1 {
			return nil, fmt.Errorf("b3: companies page %d: resposta sem totalPages", page)
		}
		total = decoded.Page.TotalPages

		for _, r := range decoded.Results {
			out = append(out, identity.CompanyRef{
				IssuingCompany: r.IssuingCompany,
				CodeCVM:        r.CodeCVM,
				CNPJ:           r.CNPJ,
			})
		}
	}
	return out, nil
}

func (p *Provider) companiesPage(ctx context.Context, page int, force bool) (companiesPage, error) {
	url, err := p.callURL("GetInitialCompanies", map[string]any{
		"language":   "pt-br",
		"pageNumber": page,
		"pageSize":   pageSize,
	})
	if err != nil {
		return companiesPage{}, err
	}

	body, err := p.client.GetRaw(ctx, url, "b3_companies", registryTTL, force)
	if err != nil {
		return companiesPage{}, err
	}

	var decoded companiesPage
	if err := json.Unmarshal(body, &decoded); err != nil {
		return companiesPage{}, fmt.Errorf("b3: companies page %d: %w", page, err)
	}
	return decoded, nil
}

// A B3 recebe o filtro como JSON em base64 no próprio path, não como query
// string.
func (p *Provider) callURL(call string, filter map[string]any) (string, error) {
	raw, err := json.Marshal(filter)
	if err != nil {
		return "", fmt.Errorf("b3: filtro de %s: %w", call, err)
	}
	return fmt.Sprintf("%s/listedCompaniesProxy/CompanyCall/%s/%s",
		p.baseURL, call, base64.StdEncoding.EncodeToString(raw)), nil
}
