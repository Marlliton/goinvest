-- name: GetMacroRate :one
SELECT value, reference_at, fetched_at FROM macro_rate WHERE id = ?;

-- name: UpsertMacroRate :exec
INSERT INTO macro_rate (id, value, reference_at, fetched_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  value        = excluded.value,
  reference_at = excluded.reference_at,
  fetched_at   = excluded.fetched_at;
