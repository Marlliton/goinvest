package app

import (
	"context"

	"github.com/marlliton/goinvest/internal/registry"
)

type RegistryConfig = registry.Config

func Registry(ctx context.Context, cfg RegistryConfig) (registry.Report, error) {
	return registry.Run(ctx, cfg)
}

type RegistryFIIConfig = registry.FIIConfig

func RegistryFII(ctx context.Context, cfg RegistryFIIConfig) (registry.Report, error) {
	return registry.RunFII(ctx, cfg)
}
