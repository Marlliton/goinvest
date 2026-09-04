-- name: ResetPeerGroups :exec
UPDATE asset SET peer_group_level = NULL, peer_group_key = NULL, peer_group_n = NULL;

-- name: ListPeerGroupPopulation :many
SELECT asset_id, class, sector, subsector, segment
FROM asset
WHERE is_active = 1 AND sector IS NOT NULL AND sector != '';

-- name: UpdateAssetPeerGroup :exec
UPDATE asset
SET peer_group_level = ?, peer_group_key = ?, peer_group_n = ?
WHERE asset_id = ?;

-- name: ClearSectorStats :exec
DELETE FROM sector_stat;

-- name: ClearAssetPercentiles :exec
DELETE FROM asset_percentile;

-- name: LatestMetricValuesForActive :many
SELECT o.asset_id, o.value
FROM (
  SELECT asset_id, value, ROW_NUMBER() OVER (
    PARTITION BY asset_id ORDER BY period_end DESC, fetched_at DESC) AS rn
  FROM observation
  WHERE metric_id = ?
) o
JOIN asset a ON a.asset_id = o.asset_id
WHERE o.rn = 1 AND a.is_active = 1 AND o.value IS NOT NULL;

-- name: InsertSectorStat :exec
INSERT INTO sector_stat (group_level, group_key, class, metric_id, n, p10, p25, median, p75, p90, computed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertAssetPercentile :exec
INSERT INTO asset_percentile (asset_id, metric_id, percentile, group_level, group_key, n, fell_back_to_market, computed_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAssetPercentiles :many
SELECT metric_id, percentile, n, fell_back_to_market
FROM asset_percentile
WHERE asset_id = ?
ORDER BY metric_id;
