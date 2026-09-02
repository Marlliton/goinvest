-- +goose Up
CREATE TABLE asset (
  ticker      TEXT PRIMARY KEY,
  class       TEXT NOT NULL,
  name        TEXT,
  updated_at  TIMESTAMP NOT NULL
);

CREATE TABLE collection_run (
  id          INTEGER PRIMARY KEY,
  source      TEXT NOT NULL,
  started_at  TIMESTAMP NOT NULL,
  finished_at TIMESTAMP,
  status      TEXT NOT NULL,
  n_obs       INTEGER,
  error       TEXT
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

CREATE TABLE raw_doc (
  url           TEXT PRIMARY KEY,
  doc_kind      TEXT NOT NULL,
  fetched_at    TIMESTAMP NOT NULL,
  etag          TEXT,
  last_modified TEXT,
  sha256        TEXT NOT NULL,
  body          BLOB NOT NULL
);

-- +goose Down
DROP TABLE raw_doc;
DROP TABLE observation;
DROP TABLE collection_run;
DROP TABLE asset;
