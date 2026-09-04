package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/marlliton/goinvest/internal/domain"
	"github.com/marlliton/goinvest/internal/store/gen"
)

// UpsertAsset atualiza classe e nome. asset é dimensão, não fato histórico:
// aqui sobrescrever é correto, diferente de observation.
func (db *DB) UpsertAsset(ctx context.Context, ticker string, class domain.AssetClass, name string, updatedAt time.Time) error {
	_, err := db.q.UpsertAsset(ctx, gen.UpsertAssetParams{
		Ticker:    ticker,
		Class:     class,
		Name:      nullString(name),
		UpdatedAt: updatedAt,
	})
	if err != nil {
		return fmt.Errorf("upsert asset %s: %w", ticker, err)
	}
	return nil
}

// GetAsset devolve found=false para ticker inexistente. Ausência não é erro:
// distinguir "nunca sincronizado" de "não existe na B3" é escopo da Fase 2.
// Ticker desconhecido cai no alias e devolve o ativo canônico, não o alias
// consultado.
func (db *DB) GetAsset(ctx context.Context, ticker string) (domain.Asset, bool, error) {
	a, err := db.q.GetAssetByTicker(ctx, ticker)
	if errors.Is(err, sql.ErrNoRows) {
		return db.getAssetByAlias(ctx, ticker)
	}
	if err != nil {
		return domain.Asset{}, false, fmt.Errorf("get asset %s: %w", ticker, err)
	}
	return assetFromRow(a), true, nil
}

func (db *DB) getAssetByAlias(ctx context.Context, aliasTicker string) (domain.Asset, bool, error) {
	assetID, err := db.q.GetAssetIDByAlias(ctx, aliasTicker)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Asset{}, false, nil
	}
	if err != nil {
		return domain.Asset{}, false, fmt.Errorf("get asset alias %s: %w", aliasTicker, err)
	}

	a, err := db.q.GetAssetByID(ctx, assetID)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Asset{}, false, nil
	}
	if err != nil {
		return domain.Asset{}, false, fmt.Errorf("get asset %d: %w", assetID, err)
	}
	return assetFromRow(a), true, nil
}

// AssetIDByTicker é a busca direta, sem passar por alias: serve a quem já sabe
// que o ticker é o canônico.
func (db *DB) AssetIDByTicker(ctx context.Context, ticker string) (int64, bool, error) {
	a, err := db.q.GetAssetByTicker(ctx, ticker)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("asset id %s: %w", ticker, err)
	}
	return a.AssetID, true, nil
}

func (db *DB) UpsertAssetAlias(ctx context.Context, aliasTicker string, assetID int64) error {
	err := db.q.UpsertAssetAlias(ctx, gen.UpsertAssetAliasParams{
		AliasTicker: aliasTicker,
		AssetID:     assetID,
	})
	if err != nil {
		return fmt.Errorf("upsert asset alias %s: %w", aliasTicker, err)
	}
	return nil
}

func assetFromRow(a gen.Asset) domain.Asset {
	return domain.Asset{
		AssetID:      a.AssetID,
		Ticker:       a.Ticker,
		Class:        a.Class,
		Name:         deref(a.Name),
		CNPJ:         deref(a.Cnpj),
		ISIN:         deref(a.Isin),
		CDCVM:        deref(a.CdCvm),
		Sector:       deref(a.Sector),
		Subsector:    deref(a.Subsector),
		Segment:      deref(a.Segment),
		SectorSrc:    deref(a.SectorSrc),
		IsActive:     a.IsActive != 0,
		LastLiquidAt: a.LastLiquidAt,
		UpdatedAt:    a.UpdatedAt,
	}
}

// UpdateAssetLiquidity só avança last_liquid_at quando o ativo está líquido:
// ficar inativo não apaga a memória de quando ele ainda negociava.
func (db *DB) UpdateAssetLiquidity(ctx context.Context, assetID int64, isActive bool, at time.Time) error {
	var active int64
	if isActive {
		active = 1
	}
	err := db.q.UpdateAssetLiquidity(ctx, gen.UpdateAssetLiquidityParams{
		IsActive:     active,
		LastLiquidAt: &at,
		AssetID:      assetID,
	})
	if err != nil {
		return fmt.Errorf("update asset liquidity %d: %w", assetID, err)
	}
	return nil
}

type AssetIdentityUpdate struct {
	AssetID   int64
	CNPJ      string
	ISIN      string
	CDCVM     string
	Sector    string
	Subsector string
	Segment   string
	SectorSrc string
	UpdatedAt time.Time
}

// UpdateAssetIdentities aplica um lote numa transação só. Quem decide o
// tamanho do lote é o chamador: é ele que sabe quanto trabalho pode perder num
// cancelamento.
func (db *DB) UpdateAssetIdentities(ctx context.Context, updates []AssetIdentityUpdate) error {
	if len(updates) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update identities: %w", err)
	}
	defer tx.Rollback()

	q := db.q.WithTx(tx)
	for _, u := range updates {
		if err := q.UpdateAssetIdentity(ctx, gen.UpdateAssetIdentityParams{
			Cnpj:      nullString(u.CNPJ),
			Isin:      nullString(u.ISIN),
			CdCvm:     nullString(u.CDCVM),
			Sector:    nullString(u.Sector),
			Subsector: nullString(u.Subsector),
			Segment:   nullString(u.Segment),
			SectorSrc: nullString(u.SectorSrc),
			UpdatedAt: u.UpdatedAt,
			AssetID:   u.AssetID,
		}); err != nil {
			return fmt.Errorf("update identity %d: %w", u.AssetID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update identities: %w", err)
	}
	return nil
}

func (db *DB) ListActiveTickers(ctx context.Context, class domain.AssetClass) ([]string, error) {
	tickers, err := db.q.ListActiveTickers(ctx, class)
	if err != nil {
		return nil, fmt.Errorf("list active tickers %s: %w", class, err)
	}
	return tickers, nil
}

// SectorCoverage é a base do aviso de cadastro incompleto: sem o total, "1.200
// setores" não diz ao usuário se falta muito ou nada.
func (db *DB) SectorCoverage(ctx context.Context, class domain.AssetClass) (total, withSector int, err error) {
	row, err := db.q.SectorCoverage(ctx, class)
	if err != nil {
		return 0, 0, fmt.Errorf("sector coverage %s: %w", class, err)
	}
	return int(row.Total), int(row.WithSector), nil
}
