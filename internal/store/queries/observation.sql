-- name: InsertObservation :exec
INSERT INTO observation
  (asset_id, metric_id, period_kind, period_end, value, unit, source, reference_at, fetched_at, run_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: LatestMetrics :many
SELECT metric_id, period_kind, period_end, value, unit, source, reference_at, fetched_at, run_id
FROM (
  SELECT *, ROW_NUMBER() OVER (
    PARTITION BY metric_id, period_kind
    ORDER BY period_end DESC, fetched_at DESC) AS rn
  FROM observation
  WHERE asset_id = ?
)
WHERE rn = 1;

-- name: HasDetailSource :one
SELECT EXISTS(
  SELECT 1 FROM observation
  WHERE asset_id = ? AND source LIKE 'fundamentus:detalhes%'
) AS has_detail;
