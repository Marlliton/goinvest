-- name: UpdateAssetIdentity :exec
UPDATE asset
SET cnpj       = ?,
    isin       = ?,
    cd_cvm     = ?,
    sector     = ?,
    subsector  = ?,
    segment    = ?,
    sector_src = ?,
    updated_at = ?
WHERE asset_id = ?;

-- name: ListActiveTickers :many
SELECT ticker FROM asset WHERE class = ? AND is_active = 1 ORDER BY ticker;

-- name: SectorCoverage :one
SELECT COUNT(*)                                                     AS total,
       COUNT(CASE WHEN sector IS NOT NULL AND sector != '' THEN 1 END) AS with_sector
FROM asset
WHERE class = ?;

-- name: ListTickersForClass :many
SELECT ticker FROM asset WHERE class = ? ORDER BY ticker;
