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

// SectorDescend lista os subsetores de um setor. BelowThreshold é do
// setor-pai, não dos subsetores: decide para onde a queda de percentil de
// cada subsetor aponta (setor-pai acima do piso vira o destino; abaixo,
// a queda continua para o mercado). SingleLevel e N cobrem a taxonomia de
// FII, que não tem subsetor: N só é populado nesse caso. AlsoFII avisa que
// o mesmo nome, além de setor de ação com subsetores, também é setor de FII.
type SectorDescend struct {
	BelowThreshold bool
	Groups         []SectorGroup
	SingleLevel    bool
	N              int
	AlsoFII        bool
}

func SectorsDescend(ctx context.Context, db *store.DB, sector string) (SectorDescend, error) {
	stockExists, err := db.SectorExists(ctx, domain.ClassStock, sector)
	if err != nil {
		return SectorDescend{}, err
	}
	fiiExists, err := db.SectorExists(ctx, domain.ClassFII, sector)
	if err != nil {
		return SectorDescend{}, err
	}
	if !stockExists && !fiiExists {
		return SectorDescend{}, ErrSectorNotFound
	}

	if !stockExists && fiiExists {
		fiiCounts, err := db.ListSectorCounts(ctx, domain.ClassFII)
		if err != nil {
			return SectorDescend{}, err
		}
		return SectorDescend{SingleLevel: true, N: sectorN(fiiCounts, sector)}, nil
	}

	stockCounts, err := db.ListSectorCounts(ctx, domain.ClassStock)
	if err != nil {
		return SectorDescend{}, err
	}

	counts, err := db.ListSubsectorCounts(ctx, domain.ClassStock, sector)
	if err != nil {
		return SectorDescend{}, err
	}
	return SectorDescend{
		BelowThreshold: sectorN(stockCounts, sector) < store.MinPeerGroup,
		Groups:         toGroups(counts),
		AlsoFII:        fiiExists,
	}, nil
}

func sectorN(counts []store.SectorCount, name string) int {
	for _, c := range counts {
		if c.Name == name {
			return c.N
		}
	}
	return 0
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
