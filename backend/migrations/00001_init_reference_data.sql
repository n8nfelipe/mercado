-- +goose Up
CREATE TABLE currencies (
    code CHAR(3) PRIMARY KEY,
    name TEXT NOT NULL,
    symbol TEXT NOT NULL,
    decimal_places SMALLINT NOT NULL CHECK (decimal_places BETWEEN 0 AND 4),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (code ~ '^[A-Z]{3}$')
);

CREATE TABLE countries (
    code CHAR(2) PRIMARY KEY,
    name TEXT NOT NULL,
    currency_code CHAR(3) NOT NULL REFERENCES currencies (code) ON UPDATE CASCADE ON DELETE RESTRICT,
    default_locale TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (code ~ '^[A-Z]{2}$'),
    CHECK (default_locale ~ '^[a-z]{2}-[A-Z]{2}$')
);

CREATE TABLE locales (
    code TEXT PRIMARY KEY,
    language TEXT NOT NULL,
    country_code CHAR(2) NOT NULL REFERENCES countries (code) ON UPDATE CASCADE ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (code ~ '^[a-z]{2}-[A-Z]{2}$'),
    CHECK (language ~ '^[a-z]{2}$')
);

CREATE INDEX countries_currency_code_idx ON countries (currency_code);
CREATE INDEX locales_country_code_idx ON locales (country_code);

-- +goose Down
DROP TABLE IF EXISTS locales;
DROP TABLE IF EXISTS countries;
DROP TABLE IF EXISTS currencies;
