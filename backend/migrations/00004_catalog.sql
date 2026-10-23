-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

CREATE TABLE categories (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    parent_id UUID REFERENCES categories (id) ON DELETE RESTRICT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(name) BETWEEN 2 AND 120),
    CHECK (slug ~ '^[a-z0-9]+(?:-[a-z0-9]+)*$'),
    CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE UNIQUE INDEX categories_slug_idx ON categories (slug);
CREATE INDEX categories_parent_id_idx ON categories (parent_id);
CREATE INDEX categories_active_idx ON categories (is_active) WHERE is_active;

CREATE TABLE listings (
    id UUID PRIMARY KEY,
    seller_id UUID NOT NULL REFERENCES seller_profiles (id) ON DELETE RESTRICT,
    category_id UUID NOT NULL REFERENCES categories (id) ON DELETE RESTRICT,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    price_cents BIGINT NOT NULL,
    currency CHAR(3) NOT NULL REFERENCES currencies (code) ON UPDATE CASCADE ON DELETE RESTRICT,
    condition TEXT NOT NULL CHECK (condition IN ('new', 'used')),
    free_shipping BOOLEAN NOT NULL DEFAULT false,
    status TEXT NOT NULL DEFAULT 'draft'
        CHECK (status IN ('draft', 'pending_review', 'published', 'rejected', 'suspended', 'archived')),
    search_text TEXT NOT NULL DEFAULT '',
    rejection_reason TEXT,
    submitted_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    archived_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (length(title) BETWEEN 10 AND 120),
    CHECK (length(description) BETWEEN 20 AND 5000),
    CHECK (price_cents > 0),
    CHECK (status <> 'published' OR published_at IS NOT NULL),
    CHECK (status <> 'rejected' OR rejection_reason IS NOT NULL)
);

CREATE INDEX listings_seller_id_idx ON listings (seller_id);
CREATE INDEX listings_category_status_idx ON listings (category_id, status);
CREATE INDEX listings_published_idx ON listings (published_at DESC, id DESC) WHERE status = 'published';
CREATE INDEX listings_search_text_trgm_idx ON listings USING gin (search_text gin_trgm_ops);
CREATE INDEX listings_price_idx ON listings (price_cents) WHERE status = 'published';

CREATE TABLE listing_images (
    id UUID PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    object_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL CHECK (content_type IN ('image/jpeg', 'image/png', 'image/webp')),
    byte_size BIGINT NOT NULL CHECK (byte_size > 0 AND byte_size <= 5242880),
    position SMALLINT NOT NULL DEFAULT 0 CHECK (position BETWEEN 0 AND 7),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX listing_images_listing_position_idx ON listing_images (listing_id, position, id);

CREATE TABLE inventory (
    listing_id UUID PRIMARY KEY REFERENCES listings (id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 0 CHECK (quantity >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE inventory_adjustments (
    id UUID PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    delta INTEGER NOT NULL CHECK (delta <> 0),
    quantity INTEGER NOT NULL CHECK (quantity >= 0),
    reason TEXT NOT NULL CHECK (length(reason) BETWEEN 3 AND 200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX inventory_adjustments_listing_created_idx ON inventory_adjustments (listing_id, created_at DESC, id DESC);

CREATE TABLE listing_moderation_events (
    id UUID PRIMARY KEY,
    listing_id UUID NOT NULL REFERENCES listings (id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    action TEXT NOT NULL CHECK (action IN ('submit', 'publish', 'reject', 'suspend', 'archive')),
    reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (action <> 'reject' OR (reason IS NOT NULL AND length(reason) >= 3))
);

CREATE INDEX listing_moderation_events_listing_created_idx ON listing_moderation_events (listing_id, created_at DESC, id DESC);

-- +goose Down
DROP TABLE IF EXISTS listing_moderation_events;
DROP TABLE IF EXISTS inventory_adjustments;
DROP TABLE IF EXISTS inventory;
DROP TABLE IF EXISTS listing_images;
DROP TABLE IF EXISTS listings;
DROP TABLE IF EXISTS categories;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pg_trgm;
