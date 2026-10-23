package store

import (
	"context"
	"errors"
	"fmt"
	"path"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"mercado/internal/search"
)

const listingColumns = `
	l.id, l.seller_id, sp.display_name, l.category_id, c.name, c.slug,
	l.title, l.description, l.price_cents, l.currency, l.condition, l.free_shipping,
	l.status, l.search_text, l.rejection_reason, COALESCE(i.quantity, 0),
	l.submitted_at, l.published_at, l.archived_at, l.created_at, l.updated_at
`

const listingFrom = `
	FROM listings l
	JOIN seller_profiles sp ON sp.id = l.seller_id
	JOIN categories c ON c.id = l.category_id
	LEFT JOIN inventory i ON i.listing_id = l.id
`

func (s *Store) CreateListing(ctx context.Context, sellerID, actorID uuid.UUID, input ListingInput) (Listing, error) {
	var active bool
	err := s.pool.QueryRow(ctx, "SELECT is_active FROM categories WHERE id = $1", input.CategoryID).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
		return Listing{}, ErrCategoryInactive
	}
	if err != nil {
		return Listing{}, fmt.Errorf("check listing category: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Listing{}, fmt.Errorf("new listing id: %w", err)
	}
	searchText := search.SearchText(input.Title, input.Description)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("begin create listing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO listings (
			id, seller_id, category_id, title, description, price_cents, currency,
			condition, free_shipping, status, search_text
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'draft', $10)
	`, id, sellerID, input.CategoryID, input.Title, input.Description, input.PriceCents,
		input.Currency, input.Condition, input.FreeShipping, searchText)
	if err != nil {
		return Listing{}, fmt.Errorf("insert listing: %w", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO inventory (listing_id, quantity) VALUES ($1, $2)", id, input.Stock); err != nil {
		return Listing{}, fmt.Errorf("insert inventory: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, fmt.Errorf("commit create listing: %w", err)
	}
	return s.ListingByID(ctx, id)
}

func (s *Store) ListingByID(ctx context.Context, id uuid.UUID) (Listing, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+listingColumns+`
		`+listingFrom+`
		WHERE l.id = $1
	`, id)
	listing, err := scanListing(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("select listing: %w", err)
	}
	images, err := s.listingImages(ctx, []uuid.UUID{id})
	if err != nil {
		return Listing{}, err
	}
	listing.Images = images[id]
	return listing, nil
}

func (s *Store) PublicListingByID(ctx context.Context, id uuid.UUID) (Listing, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+listingColumns+`
		`+listingFrom+`
		WHERE l.id = $1 AND l.status = 'published' AND c.is_active
	`, id)
	listing, err := scanListing(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("select public listing: %w", err)
	}
	images, err := s.listingImages(ctx, []uuid.UUID{id})
	if err != nil {
		return Listing{}, err
	}
	listing.Images = images[id]
	return listing, nil
}

func (s *Store) SellerListings(ctx context.Context, sellerID uuid.UUID) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+listingColumns+`
		`+listingFrom+`
		WHERE l.seller_id = $1
		ORDER BY l.updated_at DESC, l.id DESC
	`, sellerID)
	if err != nil {
		return nil, fmt.Errorf("list seller listings: %w", err)
	}
	listings, ids, err := scanListings(rows)
	if err != nil {
		return nil, err
	}
	images, err := s.listingImages(ctx, ids)
	if err != nil {
		return nil, err
	}
	for index := range listings {
		listings[index].Images = images[listings[index].ID]
	}
	return listings, nil
}

func (s *Store) PendingListings(ctx context.Context) ([]Listing, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+listingColumns+`
		`+listingFrom+`
		WHERE l.status = 'pending_review'
		ORDER BY l.submitted_at ASC, l.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list pending listings: %w", err)
	}
	listings, ids, err := scanListings(rows)
	if err != nil {
		return nil, err
	}
	images, err := s.listingImages(ctx, ids)
	if err != nil {
		return nil, err
	}
	for index := range listings {
		listings[index].Images = images[listings[index].ID]
	}
	return listings, nil
}

func (s *Store) UpdateListing(ctx context.Context, listingID, sellerID, actorID uuid.UUID, patch ListingPatch) (Listing, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("begin update listing: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var status, title, description string
	err = tx.QueryRow(ctx, `
		SELECT seller_id, status, title, description FROM listings WHERE id = $1 FOR UPDATE
	`, listingID).Scan(&ownerID, &status, &title, &description)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("lock listing: %w", err)
	}
	if ownerID != sellerID {
		return Listing{}, ErrForbidden
	}

	switch status {
	case "draft", "rejected":
	case "published", "suspended":
		if patch.Title != nil || patch.Description != nil || patch.CategoryID != nil ||
			patch.Condition != nil || patch.FreeShipping != nil {
			return Listing{}, ErrRequiresReview
		}
	default:
		return Listing{}, ErrInvalidState
	}

	if patch.Title != nil {
		title = *patch.Title
	}
	if patch.Description != nil {
		description = *patch.Description
	}
	if patch.CategoryID != nil {
		var active bool
		err := tx.QueryRow(ctx, "SELECT is_active FROM categories WHERE id = $1", *patch.CategoryID).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
			return Listing{}, ErrCategoryInactive
		}
		if err != nil {
			return Listing{}, fmt.Errorf("check patch category: %w", err)
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE listings SET
			title = COALESCE($2, title),
			description = COALESCE($3, description),
			category_id = COALESCE($4, category_id),
			price_cents = COALESCE($5, price_cents),
			condition = COALESCE($6, condition),
			free_shipping = COALESCE($7, free_shipping),
			search_text = $8,
			updated_at = now()
		WHERE id = $1
	`, listingID, patch.Title, patch.Description, patch.CategoryID, patch.PriceCents,
		patch.Condition, patch.FreeShipping, search.SearchText(title, description))
	if err != nil {
		return Listing{}, fmt.Errorf("update listing: %w", err)
	}

	if patch.Stock != nil {
		if err := s.setInventoryTx(ctx, tx, listingID, actorID, *patch.Stock, "listing update"); err != nil {
			return Listing{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return Listing{}, fmt.Errorf("commit update listing: %w", err)
	}
	return s.ListingByID(ctx, listingID)
}

func (s *Store) SubmitListing(ctx context.Context, listingID, sellerID, actorID uuid.UUID) (Listing, error) {
	return s.sellerTransition(ctx, listingID, sellerID, actorID, "submit", "", []string{"draft", "rejected"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE listings
			SET status = 'pending_review', submitted_at = now(), rejection_reason = NULL, updated_at = now()
			WHERE id = $1
		`, listingID)
		return err
	})
}

func (s *Store) ArchiveListing(ctx context.Context, listingID, sellerID, actorID uuid.UUID) (Listing, error) {
	return s.sellerTransition(ctx, listingID, sellerID, actorID, "archive", "", []string{"draft", "published", "suspended"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE listings
			SET status = 'archived', archived_at = now(), updated_at = now()
			WHERE id = $1
		`, listingID)
		return err
	})
}

func (s *Store) PublishListing(ctx context.Context, listingID, actorID uuid.UUID) (Listing, error) {
	return s.adminTransition(ctx, listingID, actorID, "publish", "", []string{"pending_review", "suspended"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE listings
			SET status = 'published', published_at = COALESCE(published_at, now()), updated_at = now()
			WHERE id = $1
		`, listingID)
		return err
	})
}

func (s *Store) RejectListing(ctx context.Context, listingID, actorID uuid.UUID, reason string) (Listing, error) {
	return s.adminTransition(ctx, listingID, actorID, "reject", reason, []string{"pending_review"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE listings
			SET status = 'rejected', rejection_reason = $2, updated_at = now()
			WHERE id = $1
		`, listingID, reason)
		return err
	})
}

func (s *Store) SuspendListing(ctx context.Context, listingID, actorID uuid.UUID, reason string) (Listing, error) {
	return s.adminTransition(ctx, listingID, actorID, "suspend", reason, []string{"published"}, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			UPDATE listings
			SET status = 'suspended', rejection_reason = $2, updated_at = now()
			WHERE id = $1
		`, listingID, reason)
		return err
	})
}

func (s *Store) sellerTransition(
	ctx context.Context,
	listingID, sellerID, actorID uuid.UUID,
	action, reason string,
	allowed []string,
	update func(pgx.Tx) error,
) (Listing, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("begin %s listing: %w", action, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, "SELECT seller_id, status FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("lock listing: %w", err)
	}
	if ownerID != sellerID {
		return Listing{}, ErrForbidden
	}
	if !containsString(allowed, status) {
		return Listing{}, ErrInvalidState
	}
	if err := update(tx); err != nil {
		return Listing{}, fmt.Errorf("update listing state: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO listing_moderation_events (id, listing_id, actor_id, action, reason)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
	`, mustUUIDv7(), listingID, actorID, action, reason); err != nil {
		return Listing{}, fmt.Errorf("insert moderation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, fmt.Errorf("commit %s listing: %w", action, err)
	}
	return s.ListingByID(ctx, listingID)
}

func (s *Store) adminTransition(
	ctx context.Context,
	listingID, actorID uuid.UUID,
	action, reason string,
	allowed []string,
	update func(pgx.Tx) error,
) (Listing, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("begin %s listing: %w", action, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var status string
	err = tx.QueryRow(ctx, "SELECT status FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("lock listing: %w", err)
	}
	if !containsString(allowed, status) {
		return Listing{}, ErrInvalidState
	}
	if err := update(tx); err != nil {
		return Listing{}, fmt.Errorf("update listing state: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO listing_moderation_events (id, listing_id, actor_id, action, reason)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
	`, mustUUIDv7(), listingID, actorID, action, reason); err != nil {
		return Listing{}, fmt.Errorf("insert moderation event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, fmt.Errorf("commit %s listing: %w", action, err)
	}
	return s.ListingByID(ctx, listingID)
}

func (s *Store) listingImages(ctx context.Context, listingIDs []uuid.UUID) (map[uuid.UUID][]ListingImage, error) {
	result := make(map[uuid.UUID][]ListingImage)
	if len(listingIDs) == 0 {
		return result, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, listing_id, object_key, content_type, byte_size, position, created_at
		FROM listing_images WHERE listing_id = ANY($1::uuid[])
		ORDER BY listing_id, position, id
	`, listingIDs)
	if err != nil {
		return nil, fmt.Errorf("list listing images: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var image ListingImage
		var listingID uuid.UUID
		if err := rows.Scan(&image.ID, &listingID, &image.ObjectKey, &image.ContentType, &image.ByteSize, &image.Position, &image.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan listing image: %w", err)
		}
		image.URL = mediaURL(listingID, image.ObjectKey)
		result[listingID] = append(result[listingID], image)
	}
	return result, rows.Err()
}

func mediaURL(listingID uuid.UUID, objectKey string) string {
	return "/v1/media/" + listingID.String() + "/" + path.Base(objectKey)
}

func scanListing(scanner categoryScanner) (Listing, error) {
	var listing Listing
	err := scanner.Scan(
		&listing.ID, &listing.SellerID, &listing.SellerName, &listing.CategoryID,
		&listing.CategoryName, &listing.CategorySlug, &listing.Title, &listing.Description,
		&listing.PriceCents, &listing.Currency, &listing.Condition, &listing.FreeShipping,
		&listing.Status, &listing.SearchText, &listing.RejectionReason, &listing.Stock,
		&listing.SubmittedAt, &listing.PublishedAt, &listing.ArchivedAt, &listing.CreatedAt, &listing.UpdatedAt,
	)
	return listing, err
}

type listingRows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close()
}

func scanListings(rows listingRows) ([]Listing, []uuid.UUID, error) {
	defer rows.Close()
	listings := make([]Listing, 0)
	ids := make([]uuid.UUID, 0)
	for rows.Next() {
		listing, err := scanListing(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("scan listing: %w", err)
		}
		listings = append(listings, listing)
		ids = append(ids, listing.ID)
	}
	return listings, ids, rows.Err()
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func mustUUIDv7() uuid.UUID {
	id, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return id
}

func (s *Store) ListingStatusCounts(ctx context.Context) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, "SELECT status, count(*) FROM listings GROUP BY status")
	if err != nil {
		return nil, fmt.Errorf("count listings by status: %w", err)
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var total int
		if err := rows.Scan(&status, &total); err != nil {
			return nil, fmt.Errorf("scan listing status count: %w", err)
		}
		counts[status] = total
	}
	return counts, rows.Err()
}
