-- +goose Up
ALTER TABLE asset ADD COLUMN is_active INTEGER NOT NULL DEFAULT 1;
ALTER TABLE asset ADD COLUMN last_liquid_at DATE;

-- +goose Down
ALTER TABLE asset DROP COLUMN last_liquid_at;
ALTER TABLE asset DROP COLUMN is_active;
