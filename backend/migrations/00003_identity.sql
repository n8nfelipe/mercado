-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(name) BETWEEN 2 AND 120),
    CHECK (length(email) BETWEEN 3 AND 254),
    CHECK (email_normalized = lower(email_normalized))
);

CREATE UNIQUE INDEX users_email_normalized_idx ON users (email_normalized);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('buyer', 'seller', 'admin')),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (expires_at > created_at)
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE seller_profiles (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL UNIQUE REFERENCES users (id) ON DELETE CASCADE,
    display_name TEXT NOT NULL,
    document TEXT NOT NULL,
    document_normalized TEXT NOT NULL,
    contact TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(display_name) BETWEEN 2 AND 120),
    CHECK (length(document) BETWEEN 11 AND 32),
    CHECK (length(contact) BETWEEN 5 AND 254)
);

CREATE UNIQUE INDEX seller_profiles_document_normalized_idx ON seller_profiles (document_normalized);

-- +goose Down
DROP TABLE IF EXISTS seller_profiles;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS users;
