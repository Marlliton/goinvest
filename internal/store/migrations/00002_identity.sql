-- +goose Up
DROP TABLE observation;
DROP TABLE asset;

CREATE TABLE asset (
  asset_id    INTEGER PRIMARY KEY,
  ticker      TEXT NOT NULL,
  class       TEXT NOT NULL,
  name        TEXT,
  cnpj        TEXT,
  isin        TEXT,
  cd_cvm      TEXT,
  sector      TEXT,
  subsector   TEXT,
  segment     TEXT,
  sector_src  TEXT,
  updated_at  TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX ux_asset_ticker ON asset(ticker);

CREATE TABLE asset_alias (
  alias_ticker TEXT PRIMARY KEY,
  asset_id     INTEGER NOT NULL REFERENCES asset(asset_id)
);

CREATE TABLE observation (
  id            INTEGER PRIMARY KEY,
  asset_id      INTEGER NOT NULL REFERENCES asset(asset_id),
  metric_id     TEXT NOT NULL,
  period_kind   TEXT NOT NULL DEFAULT 'spot',
  period_end    DATE NOT NULL,
  value         REAL,
  unit          TEXT NOT NULL,
  source        TEXT NOT NULL,
  reference_at  DATE,
  fetched_at    TIMESTAMP NOT NULL,
  run_id        INTEGER REFERENCES collection_run(id)
);
CREATE INDEX ix_obs_lookup ON observation(asset_id, metric_id, period_kind, period_end DESC);
CREATE UNIQUE INDEX ux_obs ON observation(asset_id, metric_id, period_kind, period_end, source, fetched_at);

-- +goose Down
DROP TABLE observation;
DROP TABLE asset_alias;
DROP TABLE asset;

CREATE TABLE asset (
  ticker      TEXT PRIMARY KEY,
  class       TEXT NOT NULL,
  name        TEXT,
  updated_at  TIMESTAMP NOT NULL
);

CREATE TABLE observation (
  id            INTEGER PRIMARY KEY,
  ticker        TEXT NOT NULL REFERENCES asset(ticker),
  metric_id     TEXT NOT NULL,
  period_kind   TEXT NOT NULL DEFAULT 'spot',
  period_end    DATE NOT NULL,
  value         REAL,
  unit          TEXT NOT NULL,
  source        TEXT NOT NULL,
  reference_at  DATE,
  fetched_at    TIMESTAMP NOT NULL,
  run_id        INTEGER REFERENCES collection_run(id)
);
CREATE INDEX ix_obs_lookup ON observation(ticker, metric_id, period_kind, period_end DESC);
CREATE UNIQUE INDEX ux_obs ON observation(ticker, metric_id, period_kind, period_end, source, fetched_at);
