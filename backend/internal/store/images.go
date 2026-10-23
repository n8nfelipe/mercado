package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CanAddListingImage(ctx context.Context, listingID, sellerID uuid.UUID) error {
	var ownerID uuid.UUID
	var status string
	err := s.pool.QueryRow(ctx, "SELECT seller_id, status FROM listings WHERE id = $1", listingID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrListingNotFound
	}
	if err != nil {
		return fmt.Errorf("check listing image permission: %w", err)
	}
	if ownerID != sellerID {
		return ErrForbidden
	}
	if status == "pending_review" || status == "archived" {
		return ErrInvalidState
	}
	var count int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM listing_images WHERE listing_id = $1", listingID).Scan(&count); err != nil {
		return fmt.Errorf("count listing images: %w", err)
	}
	if count >= 8 {
		return ErrImageLimit
	}
	return nil
}

func (s *Store) AddListingImage(ctx context.Context, listingID, sellerID, imageID uuid.UUID, objectKey, contentType string, byteSize int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin add image: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, "SELECT seller_id, status FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrListingNotFound
	}
	if err != nil {
		return fmt.Errorf("lock listing for image: %w", err)
	}
	if ownerID != sellerID {
		return ErrForbidden
	}
	if status == "pending_review" || status == "archived" {
		return ErrInvalidState
	}

	var count int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM listing_images WHERE listing_id = $1", listingID).Scan(&count); err != nil {
		return fmt.Errorf("count listing images: %w", err)
	}
	if count >= 8 {
		return ErrImageLimit
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO listing_images (id, listing_id, object_key, content_type, byte_size, position)
		SELECT $1, $2, $3, $4, $5, COALESCE(max(position) + 1, 0)
		FROM listing_images WHERE listing_id = $2
	`, imageID, listingID, objectKey, contentType, byteSize)
	if err != nil {
		return fmt.Errorf("insert listing image: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Store) DeleteListingImage(ctx context.Context, listingID, sellerID, imageID uuid.UUID) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin delete image: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, "SELECT seller_id, status FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrListingNotFound
	}
	if err != nil {
		return "", fmt.Errorf("lock listing for image delete: %w", err)
	}
	if ownerID != sellerID {
		return "", ErrForbidden
	}
	if status == "pending_review" || status == "archived" {
		return "", ErrInvalidState
	}

	var objectKey string
	err = tx.QueryRow(ctx, "SELECT object_key FROM listing_images WHERE id = $1 AND listing_id = $2", imageID, listingID).Scan(&objectKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrImageNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select listing image: %w", err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM listing_images WHERE id = $1", imageID); err != nil {
		return "", fmt.Errorf("delete listing image: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		WITH ranked AS (
			SELECT id, row_number() OVER (ORDER BY position, id) - 1 AS new_position
			FROM listing_images WHERE listing_id = $1
		)
		UPDATE listing_images li SET position = ranked.new_position
		FROM ranked WHERE li.id = ranked.id
	`, listingID); err != nil {
		return "", fmt.Errorf("compact listing image positions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit delete image: %w", err)
	}
	return objectKey, nil
}

func (s *Store) ReorderListingImages(ctx context.Context, listingID, sellerID uuid.UUID, imageIDs []uuid.UUID) (Listing, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Listing{}, fmt.Errorf("begin reorder images: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	var status string
	err = tx.QueryRow(ctx, "SELECT seller_id, status FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&ownerID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Listing{}, ErrListingNotFound
	}
	if err != nil {
		return Listing{}, fmt.Errorf("lock listing for reorder: %w", err)
	}
	if ownerID != sellerID {
		return Listing{}, ErrForbidden
	}
	if status == "pending_review" || status == "archived" {
		return Listing{}, ErrInvalidState
	}

	rows, err := tx.Query(ctx, "SELECT id FROM listing_images WHERE listing_id = $1", listingID)
	if err != nil {
		return Listing{}, fmt.Errorf("list image ids: %w", err)
	}
	existing := make(map[uuid.UUID]bool)
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return Listing{}, fmt.Errorf("scan image id: %w", err)
		}
		existing[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Listing{}, err
	}

	seen := make(map[uuid.UUID]bool)
	if len(imageIDs) != len(existing) {
		return Listing{}, ErrInvalidListingData
	}
	for _, id := range imageIDs {
		if !existing[id] || seen[id] {
			return Listing{}, ErrInvalidListingData
		}
		seen[id] = true
	}
	for position, id := range imageIDs {
		if _, err := tx.Exec(ctx, "UPDATE listing_images SET position = $3 WHERE id = $1 AND listing_id = $2", id, listingID, position); err != nil {
			return Listing{}, fmt.Errorf("reorder image: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Listing{}, fmt.Errorf("commit reorder images: %w", err)
	}
	return s.ListingByID(ctx, listingID)
}

func (s *Store) ImageContentTypeByObjectKey(ctx context.Context, objectKey string) (string, error) {
	var contentType string
	err := s.pool.QueryRow(ctx, "SELECT content_type FROM listing_images WHERE object_key = $1", objectKey).Scan(&contentType)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrImageNotFound
	}
	if err != nil {
		return "", fmt.Errorf("select image object: %w", err)
	}
	return contentType, nil
}
