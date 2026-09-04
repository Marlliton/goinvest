package app

import (
	"context"

	"github.com/marlliton/goinvest/internal/registry"
)

type RegistryConfig = registry.Config

func Registry(ctx context.Context, cfg RegistryConfig) (registry.Report, error) {
	return registry.Run(ctx, cfg)
}
