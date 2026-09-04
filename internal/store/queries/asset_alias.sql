-- name: UpsertAssetAlias :exec
INSERT INTO asset_alias (alias_ticker, asset_id)
VALUES (?, ?)
ON CONFLICT(alias_ticker) DO UPDATE SET
  asset_id = excluded.asset_id;

-- name: GetAssetIDByAlias :one
SELECT asset_id FROM asset_alias WHERE alias_ticker = ?;
