-- +goose Up
CREATE TABLE dividend_event (
  id            INTEGER PRIMARY KEY,
  asset_id      INTEGER NOT NULL REFERENCES asset(asset_id),
  ex_date       DATE NOT NULL,
  payment_date  DATE,
  type          TEXT NOT NULL,
  type_raw      TEXT NOT NULL,
  value_raw     REAL NOT NULL,
  shares_factor REAL NOT NULL,
  source        TEXT NOT NULL,
  fetched_at    TIMESTAMP NOT NULL
);

-- A chave natural é a linha inteira que a fonte publica: mesma data-com e
-- mesmo valor com datas de pagamento diferentes são parcelas distintas.
-- COALESCE porque NULL não colide com NULL em índice único, e a maioria dos
-- eventos antigos não tem data de pagamento.
CREATE UNIQUE INDEX ux_dividend_event ON dividend_event(
  asset_id, ex_date, COALESCE(payment_date, ''), type_raw, value_raw, source);

CREATE INDEX ix_dividend_event_lookup ON dividend_event(asset_id, ex_date DESC);

-- +goose Down
DROP INDEX ix_dividend_event_lookup;
DROP INDEX ux_dividend_event;
DROP TABLE dividend_event;
