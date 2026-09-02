-- name: UpsertAsset :exec
INSERT INTO asset (ticker, class, name, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(ticker) DO UPDATE SET
  class      = excluded.class,
  name       = excluded.name,
  updated_at = excluded.updated_at;

-- name: GetAsset :one
SELECT ticker, class, name, updated_at
FROM asset
WHERE ticker = ?;
