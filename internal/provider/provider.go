package provider

import (
	"context"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
)

type Namer interface {
	Name() string
}

// force ignora o TTL de cache do fetch.
type UniverseProvider interface {
	Namer
	SourceID(class domain.AssetClass) string
	Universe(ctx context.Context, class domain.AssetClass, force bool) ([]domain.Observation, error)
}

type SelicProvider interface {
	Namer
	Selic(ctx context.Context, force bool) (rate float64, referenceAt time.Time, err error)
}

type IdentityProvider interface {
	Namer
	Companies(ctx context.Context, force bool) ([]identity.CompanyRef, error)
	Detail(ctx context.Context, codeCVM string, force bool) (identity.CompanyDetail, error)
}

type DetailProvider interface {
	Namer
	Detail(ctx context.Context, ticker string, class domain.AssetClass, force bool) (domain.MetricSet, error)
	Dividends(ctx context.Context, ticker string, class domain.AssetClass, force bool) ([]domain.DividendEvent, error)
}

type FIIISINProvider interface {
	Namer
	ISINByCNPJ(ctx context.Context, force bool) (map[string]string, error)
}

type FIISegmentProvider interface {
	Namer
	Segments(ctx context.Context, force bool) (map[string]string, error)
}
