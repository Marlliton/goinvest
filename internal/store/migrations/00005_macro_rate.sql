-- +goose Up
CREATE TABLE macro_rate (
  id           TEXT PRIMARY KEY,
  value        REAL NOT NULL,
  reference_at DATE NOT NULL,
  fetched_at   TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE macro_rate;
