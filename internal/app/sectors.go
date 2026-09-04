package app

import (
	"context"
	"errors"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store"
)

var ErrSectorNotFound = errors.New("setor não encontrado")

type SectorGroup struct {
	Name           string
	N              int
	BelowThreshold bool
}

type ClassSectors struct {
	Class              domain.AssetClass
	Groups             []SectorGroup
	IncompleteRegistry int
	TotalAssets        int
}

func Sectors(ctx context.Context, db *store.DB) ([]ClassSectors, error) {
	classes := []domain.AssetClass{domain.ClassStock, domain.ClassFII}

	out := make([]ClassSectors, 0, len(classes))
	for _, class := range classes {
		counts, err := db.ListSectorCounts(ctx, class)
		if err != nil {
			return nil, err
		}
		total, withSector, err := db.SectorCoverage(ctx, class)
		if err != nil {
			return nil, err
		}
		out = append(out, ClassSectors{
			Class:              class,
			Groups:             toGroups(counts),
			IncompleteRegistry: total - withSector,
			TotalAssets:        total,
		})
	}
	return out, nil
}

func SectorsDescend(ctx context.Context, db *store.DB, sector string) ([]SectorGroup, error) {
	exists, err := db.SectorExists(ctx, domain.ClassStock, sector)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrSectorNotFound
	}

	counts, err := db.ListSubsectorCounts(ctx, domain.ClassStock, sector)
	if err != nil {
		return nil, err
	}
	return toGroups(counts), nil
}

func toGroups(counts []store.SectorCount) []SectorGroup {
	out := make([]SectorGroup, 0, len(counts))
	for _, c := range counts {
		out = append(out, SectorGroup{
			Name:           c.Name,
			N:              c.N,
			BelowThreshold: c.N < store.MinPeerGroup,
		})
	}
	return out
}
