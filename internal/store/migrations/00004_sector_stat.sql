-- +goose Up
ALTER TABLE asset ADD COLUMN peer_group_level TEXT;
ALTER TABLE asset ADD COLUMN peer_group_key TEXT;
ALTER TABLE asset ADD COLUMN peer_group_n INTEGER;

CREATE TABLE sector_stat (
  group_level TEXT NOT NULL,
  group_key   TEXT NOT NULL,
  class       TEXT NOT NULL,
  metric_id   TEXT NOT NULL,
  n           INTEGER NOT NULL,
  p10         REAL,
  p25         REAL,
  median      REAL,
  p75         REAL,
  p90         REAL,
  computed_at TIMESTAMP NOT NULL,
  PRIMARY KEY (group_level, group_key, class, metric_id)
);

CREATE TABLE asset_percentile (
  asset_id            INTEGER NOT NULL REFERENCES asset(asset_id),
  metric_id           TEXT NOT NULL,
  percentile          REAL NOT NULL,
  group_level         TEXT NOT NULL,
  group_key           TEXT NOT NULL,
  n                   INTEGER NOT NULL,
  fell_back_to_market INTEGER NOT NULL DEFAULT 0,
  computed_at         TIMESTAMP NOT NULL,
  PRIMARY KEY (asset_id, metric_id)
);

-- +goose Down
DROP TABLE asset_percentile;
DROP TABLE sector_stat;
ALTER TABLE asset DROP COLUMN peer_group_n;
ALTER TABLE asset DROP COLUMN peer_group_key;
ALTER TABLE asset DROP COLUMN peer_group_level;
