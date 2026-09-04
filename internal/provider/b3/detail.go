package b3

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/marlliton/goinvest/internal/identity"
)

type companyDetail struct {
	Code                   string `json:"code"`
	CodeCVM                string `json:"codeCVM"`
	CNPJ                   string `json:"cnpj"`
	IndustryClassification string `json:"industryClassification"`
	OtherCodes             []struct {
		Code string `json:"code"`
		ISIN string `json:"isin"`
	} `json:"otherCodes"`
}

// Detail devolve a identidade e a taxonomia de três níveis de uma companhia.
// Um codeCVM cobre todas as classes de ação dela.
func (p *Provider) Detail(ctx context.Context, codeCVM string, force bool) (identity.CompanyDetail, error) {
	url, err := p.callURL("GetDetail", map[string]any{
		"codeCVM":  codeCVM,
		"language": "pt-br",
	})
	if err != nil {
		return identity.CompanyDetail{}, err
	}

	body, err := p.client.GetRaw(ctx, url, "b3_detail", registryTTL, force)
	if err != nil {
		return identity.CompanyDetail{}, err
	}

	var decoded companyDetail
	if err := json.Unmarshal(body, &decoded); err != nil {
		return identity.CompanyDetail{}, fmt.Errorf("b3: detail %s: %w", codeCVM, err)
	}
	// Setor vazio gravado como dado real contamina toda a referência
	// estatística construída em cima dele.
	if decoded.IndustryClassification == "" {
		return identity.CompanyDetail{}, fmt.Errorf("b3: detail %s: industryClassification ausente", codeCVM)
	}

	out := identity.CompanyDetail{
		Code:                   decoded.Code,
		CodeCVM:                decoded.CodeCVM,
		CNPJ:                   decoded.CNPJ,
		IndustryClassification: decoded.IndustryClassification,
		OtherCodes:             make([]identity.CompanyCode, 0, len(decoded.OtherCodes)),
	}
	for _, c := range decoded.OtherCodes {
		out.OtherCodes = append(out.OtherCodes, identity.CompanyCode{Code: c.Code, ISIN: c.ISIN})
	}
	return out, nil
}
