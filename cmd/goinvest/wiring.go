package main

import (
	"context"
	"fmt"
	"time"

	"github.com/adrg/xdg"
	"github.com/marlliton/goinvest/internal/catalog"
	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/fetch"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/provider/b3"
	"github.com/marlliton/goinvest/internal/provider/cvm"
	"github.com/marlliton/goinvest/internal/provider/fundamentus"
	"github.com/marlliton/goinvest/internal/store"
)

const (
	userAgent          = "goinvest/0.1 (+https://github.com/marlliton/goinvest)"
	fundamentusBaseURL = "https://www.fundamentus.com.br"
	b3BaseURL          = "https://sistemaswebb3-listados.b3.com.br"
	cvmBaseURL         = "https://dados.cvm.gov.br/dados/FII/DOC/INF_MENSAL/DADOS"
	rateEvery          = 2 * time.Second
)

var cvmYears = []int{2024, 2025, 2026}

type rootDeps struct {
	DB          *store.DB
	Catalog     *catalog.Catalog
	Providers   map[domain.AssetClass]provider.UniverseProvider
	B3          provider.IdentityProvider
	CVM         provider.FIIISINProvider
	Fundamentus provider.FIISegmentProvider
}

type dbCache struct{ db *store.DB }

func (c dbCache) Get(ctx context.Context, url string) ([]byte, time.Time, bool, error) {
	return c.db.GetRawDoc(ctx, url)
}

func (c dbCache) Put(ctx context.Context, url, docKind string, body []byte, fetchedAt time.Time) error {
	return c.db.PutRawDoc(ctx, url, docKind, body, fetchedAt)
}

func build() (rootDeps, error) {
	// DataFile, não CacheFile: um limpador de cache do sistema apagaria anos
	// de série histórica.
	path, err := xdg.DataFile("goinvest/goinvest.db")
	if err != nil {
		return rootDeps{}, fmt.Errorf("caminho do banco: %w", err)
	}

	db, err := store.Open(path)
	if err != nil {
		return rootDeps{}, err
	}

	cat, err := catalog.Load()
	if err != nil {
		db.Close()
		return rootDeps{}, err
	}

	client := fetch.NewClient(fetch.Config{
		UserAgent: userAgent,
		RateEvery: rateEvery,
		Cache:     dbCache{db},
		Now:       time.Now,
	})

	p := fundamentus.NewProvider(client, fundamentusBaseURL, time.Now)

	// O mesmo cliente serve as duas fontes: rate limit e cache são política do
	// projeto, não de cada fonte.
	return rootDeps{
		DB:      db,
		Catalog: cat,
		Providers: map[domain.AssetClass]provider.UniverseProvider{
			domain.ClassStock: p,
			domain.ClassFII:   p,
		},
		B3: b3.NewProvider(client, b3BaseURL, time.Now),
		// Um fundo grande pode faltar num ano isolado do informe: os anos são
		// combinados justamente por isso.
		CVM:         cvm.NewProvider(client, cvmBaseURL, cvmYears, time.Now),
		Fundamentus: p,
	}, nil
}
