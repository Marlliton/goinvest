-- name: InsertDividendEvent :exec
INSERT INTO dividend_event
  (asset_id, ex_date, payment_date, type, type_raw, value_raw, shares_factor, source, fetched_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(asset_id, ex_date, COALESCE(payment_date, ''), type_raw, value_raw, source)
DO NOTHING;

-- name: ListDividendEvents :many
SELECT ex_date, payment_date, type, type_raw, value_raw, shares_factor, source, fetched_at
FROM dividend_event
WHERE asset_id = ?
ORDER BY ex_date DESC;
