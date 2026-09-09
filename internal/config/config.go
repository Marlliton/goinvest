package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Tributacao struct {
	AliquotaAtivo     float64
	AliquotaRendaFixa float64
}

type Result struct {
	Tributacao Tributacao
	Path       string
	FromFile   bool
}

// Lei 15.270/2025: dividendo pago por pessoa jurídica a pessoa física é
// isento de imposto de renda até R$ 50 mil por mês.
const defaultAliquotaAtivo = 0.0

// Tabela regressiva do imposto de renda sobre renda fixa: 15% para
// aplicações com prazo acima de 720 dias.
const defaultAliquotaRendaFixa = 0.15

func Default() Tributacao {
	return Tributacao{
		AliquotaAtivo:     defaultAliquotaAtivo,
		AliquotaRendaFixa: defaultAliquotaRendaFixa,
	}
}

type rawConfig struct {
	Tributacao rawTributacao `toml:"tributacao"`
}

type rawTributacao struct {
	AliquotaAtivo     float64 `toml:"aliquota_ativo"`
	AliquotaRendaFixa float64 `toml:"aliquota_renda_fixa"`
}

func Load(path string) (Result, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{Tributacao: Default(), Path: path, FromFile: false}, nil
		}
		return Result{}, fmt.Errorf("config: %s: %w", path, err)
	}

	var raw rawConfig
	meta, err := toml.DecodeFile(path, &raw)
	if err != nil {
		return Result{}, fmt.Errorf("config: %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		return Result{}, fmt.Errorf("config: %s: campo desconhecido %v", path, undecoded)
	}

	if err := validateAliquota("aliquota_ativo", raw.Tributacao.AliquotaAtivo); err != nil {
		return Result{}, fmt.Errorf("config: %s: %w", path, err)
	}
	if err := validateAliquota("aliquota_renda_fixa", raw.Tributacao.AliquotaRendaFixa); err != nil {
		return Result{}, fmt.Errorf("config: %s: %w", path, err)
	}

	return Result{
		Tributacao: Tributacao{
			AliquotaAtivo:     raw.Tributacao.AliquotaAtivo,
			AliquotaRendaFixa: raw.Tributacao.AliquotaRendaFixa,
		},
		Path:     path,
		FromFile: true,
	}, nil
}

func validateAliquota(campo string, valor float64) error {
	if valor < 0 || valor > 1 {
		return fmt.Errorf("%s=%v fora de 0-1", campo, valor)
	}
	return nil
}
