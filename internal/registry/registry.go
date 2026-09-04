// Package registry resolve quem é o ativo: casa o ticker com o cadastro de
// companhias abertas e grava identidade e setor oficial.
package registry

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/identity"
	"github.com/marlliton/goinvest/internal/norm"
	"github.com/marlliton/goinvest/internal/provider"
	"github.com/marlliton/goinvest/internal/store"
)

const (
	StatusOK        = "ok"
	StatusPartial   = "partial"
	StatusCancelled = "cancelled"
)

const (
	sourceID          = "b3:listed_companies"
	sectorSrc         = "b3"
	defaultBatchSize  = 50
	classificationSep = " / "
	classificationLen = 3
)

type Progress struct {
	Done  int
	Total int
}

type Config struct {
	DB       *store.DB
	Identity provider.IdentityProvider
	Force    bool
	// Lote menor perde menos trabalho num cancelamento e commita mais vezes.
	BatchSize  int
	Now        func() time.Time
	OnProgress func(Progress)
}

type Report struct {
	Total     int
	Matched   int
	Unmatched int
	Cancelled bool
	Status    string
}

// Run processa o universo inteiro a cada chamada: não há estado de retomada, e
// o cache HTTP é o que torna a segunda rodada barata.
func Run(ctx context.Context, cfg Config) (Report, error) {
	if cfg.DB == nil {
		return Report{}, errors.New("registry: db is required")
	}
	if cfg.Identity == nil {
		return Report{}, errors.New("registry: identity provider is required")
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = defaultBatchSize
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	tickers, err := cfg.DB.ListActiveTickers(ctx, domain.ClassStock)
	if err != nil {
		return Report{}, err
	}
	companies, err := cfg.Identity.Companies(ctx, cfg.Force)
	if err != nil {
		return Report{}, err
	}

	runID, err := cfg.DB.StartRun(ctx, sourceID)
	if err != nil {
		return Report{}, err
	}

	report := Report{Total: len(tickers)}
	batch := make([]store.AssetIdentityUpdate, 0, cfg.BatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		// O lote vai com um contexto próprio: o ctx cancelado abortaria a
		// gravação do trabalho que já foi feito.
		if err := cfg.DB.UpdateAssetIdentities(context.WithoutCancel(ctx), batch); err != nil {
			return err
		}
		report.Matched += len(batch)
		batch = batch[:0]
		if cfg.OnProgress != nil {
			cfg.OnProgress(Progress{Done: report.Matched + report.Unmatched, Total: report.Total})
		}
		return nil
	}

	for _, ticker := range tickers {
		if ctx.Err() != nil {
			report.Cancelled = true
			break
		}

		update, ok := resolve(ctx, cfg, companies, ticker)
		if !ok {
			// Cancelar no meio de um ticker não é o mesmo que não achá-lo:
			// contá-lo como sem correspondência faria o relatório mentir.
			if ctx.Err() != nil {
				report.Cancelled = true
				break
			}
			report.Unmatched++
			continue
		}
		batch = append(batch, update)

		if len(batch) >= cfg.BatchSize {
			if err := flush(); err != nil {
				return report, err
			}
		}
	}

	if err := flush(); err != nil {
		return report, err
	}

	report.Status = status(report)
	// O fechamento do run não pode depender do contexto que acabou de ser
	// cancelado, senão o Ctrl-C deixaria o run aberto para sempre.
	if err := cfg.DB.FinishRun(context.WithoutCancel(ctx), runID, report.Status, report.Matched, ""); err != nil {
		return report, err
	}
	return report, nil
}

func status(r Report) string {
	switch {
	case r.Cancelled:
		return StatusCancelled
	case r.Unmatched > 0:
		return StatusPartial
	default:
		return StatusOK
	}
}

// Falha de um ticker nunca aborta o cadastro: sem correspondência é resultado,
// não erro.
func resolve(ctx context.Context, cfg Config, companies []identity.CompanyRef, ticker string) (store.AssetIdentityUpdate, bool) {
	codeCVM, ok := identity.MatchByRoot(companies, ticker)
	if !ok {
		return store.AssetIdentityUpdate{}, false
	}

	detail, err := cfg.Identity.Detail(ctx, codeCVM, cfg.Force)
	if err != nil {
		return store.AssetIdentityUpdate{}, false
	}
	isin, ok := confirmTicker(detail, ticker)
	if !ok {
		return store.AssetIdentityUpdate{}, false
	}

	parts := strings.Split(detail.IndustryClassification, classificationSep)
	if len(parts) != classificationLen {
		return store.AssetIdentityUpdate{}, false
	}

	assetID, found, err := cfg.DB.AssetIDByTicker(ctx, ticker)
	if err != nil || !found {
		return store.AssetIdentityUpdate{}, false
	}

	return store.AssetIdentityUpdate{
		AssetID:   assetID,
		CNPJ:      detail.CNPJ,
		ISIN:      isin,
		CDCVM:     detail.CodeCVM,
		Sector:    norm.CleanSector(parts[0]),
		Subsector: norm.CleanSector(parts[1]),
		Segment:   norm.CleanSector(parts[2]),
		SectorSrc: sectorSrc,
		UpdatedAt: cfg.Now(),
	}, true
}

// Casar por raiz alfabética colide: WEGE4 não existe, mas a raiz WEGE bate.
// Só otherCodes prova que o código é mesmo desta empresa.
func confirmTicker(detail identity.CompanyDetail, ticker string) (isin string, ok bool) {
	for _, c := range detail.OtherCodes {
		if c.Code == ticker {
			return c.ISIN, true
		}
	}
	if detail.Code == ticker {
		return "", true
	}
	return "", false
}
