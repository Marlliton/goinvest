-- name: UpsertAsset :one
INSERT INTO asset (ticker, class, name, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(ticker) DO UPDATE SET
  class      = excluded.class,
  name       = excluded.name,
  updated_at = excluded.updated_at
RETURNING asset_id;

-- name: GetAssetByTicker :one
SELECT asset_id, ticker, class, name, cnpj, isin, cd_cvm, sector, subsector, segment, sector_src, updated_at
FROM asset
WHERE ticker = ?;

-- name: GetAssetByID :one
SELECT asset_id, ticker, class, name, cnpj, isin, cd_cvm, sector, subsector, segment, sector_src, updated_at
FROM asset
WHERE asset_id = ?;

-- name: ListAssetIDsByTicker :many
SELECT asset_id, ticker FROM asset;
