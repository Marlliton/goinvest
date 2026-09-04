-- name: UpsertAsset :one
INSERT INTO asset (ticker, class, name, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(ticker) DO UPDATE SET
  class      = excluded.class,
  name       = excluded.name,
  updated_at = excluded.updated_at
RETURNING asset_id;

-- name: GetAssetByTicker :one
SELECT asset_id, ticker, class, name, cnpj, isin, cd_cvm, sector, subsector, segment, sector_src, updated_at, is_active, last_liquid_at, peer_group_level, peer_group_key, peer_group_n
FROM asset
WHERE ticker = ?;

-- name: GetAssetByID :one
SELECT asset_id, ticker, class, name, cnpj, isin, cd_cvm, sector, subsector, segment, sector_src, updated_at, is_active, last_liquid_at, peer_group_level, peer_group_key, peer_group_n
FROM asset
WHERE asset_id = ?;

-- name: ListAssetIDsByTicker :many
SELECT asset_id, ticker FROM asset;

-- name: UpdateAssetLiquidity :exec
UPDATE asset
SET is_active      = ?1,
    last_liquid_at = CASE WHEN ?1 = 1 THEN ?2 ELSE last_liquid_at END
WHERE asset_id = ?3;
