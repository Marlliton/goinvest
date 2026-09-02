package app

import (
	"context"

	"github.com/marlliton/goinvest/internal/collect"
)

type SyncConfig = collect.Config

func Sync(ctx context.Context, cfg SyncConfig) (collect.Report, error) {
	return collect.Sync(ctx, cfg)
}
