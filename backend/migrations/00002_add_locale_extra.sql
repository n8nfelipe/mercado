-- +goose Up
ALTER TABLE locales ADD COLUMN is_extra BOOLEAN NOT NULL DEFAULT false;
CREATE INDEX locales_is_extra_idx ON locales (is_extra) WHERE is_extra;

-- +goose Down
DROP INDEX IF EXISTS locales_is_extra_idx;
ALTER TABLE locales DROP COLUMN IF EXISTS is_extra;
