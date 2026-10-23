package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type InventoryAdjustment struct {
	ID        uuid.UUID
	ListingID uuid.UUID
	ActorID   uuid.UUID
	Delta     int
	Quantity  int
	Reason    string
	CreatedAt time.Time
}

func (s *Store) AdjustInventory(ctx context.Context, listingID, sellerID, actorID uuid.UUID, delta int, reason string) (InventoryAdjustment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return InventoryAdjustment{}, fmt.Errorf("begin inventory adjustment: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var ownerID uuid.UUID
	err = tx.QueryRow(ctx, "SELECT seller_id FROM listings WHERE id = $1 FOR UPDATE", listingID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return InventoryAdjustment{}, ErrListingNotFound
	}
	if err != nil {
		return InventoryAdjustment{}, fmt.Errorf("lock listing for inventory: %w", err)
	}
	if ownerID != sellerID {
		return InventoryAdjustment{}, ErrForbidden
	}

	adjustment, err := s.adjustInventoryTx(ctx, tx, listingID, actorID, delta, reason)
	if err != nil {
		return InventoryAdjustment{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return InventoryAdjustment{}, fmt.Errorf("commit inventory adjustment: %w", err)
	}
	return adjustment, nil
}

func (s *Store) InventoryAdjustments(ctx context.Context, listingID, sellerID uuid.UUID) ([]InventoryAdjustment, error) {
	var ownerID uuid.UUID
	err := s.pool.QueryRow(ctx, "SELECT seller_id FROM listings WHERE id = $1", listingID).Scan(&ownerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrListingNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("select listing owner: %w", err)
	}
	if ownerID != sellerID {
		return nil, ErrForbidden
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, listing_id, actor_id, delta, quantity, reason, created_at
		FROM inventory_adjustments
		WHERE listing_id = $1
		ORDER BY created_at DESC, id DESC
	`, listingID)
	if err != nil {
		return nil, fmt.Errorf("list inventory adjustments: %w", err)
	}
	defer rows.Close()

	adjustments := make([]InventoryAdjustment, 0)
	for rows.Next() {
		var adjustment InventoryAdjustment
		if err := rows.Scan(&adjustment.ID, &adjustment.ListingID, &adjustment.ActorID, &adjustment.Delta,
			&adjustment.Quantity, &adjustment.Reason, &adjustment.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan inventory adjustment: %w", err)
		}
		adjustments = append(adjustments, adjustment)
	}
	return adjustments, rows.Err()
}

func (s *Store) adjustInventoryTx(ctx context.Context, tx pgx.Tx, listingID, actorID uuid.UUID, delta int, reason string) (InventoryAdjustment, error) {
	if delta == 0 {
		return InventoryAdjustment{}, ErrInvalidListingData
	}

	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM listings WHERE id = $1", listingID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InventoryAdjustment{}, ErrListingNotFound
		}
		return InventoryAdjustment{}, fmt.Errorf("select listing for inventory: %w", err)
	}
	if status == "archived" {
		return InventoryAdjustment{}, ErrInvalidState
	}

	var quantity int
	if err := tx.QueryRow(ctx, "SELECT quantity FROM inventory WHERE listing_id = $1 FOR UPDATE", listingID).Scan(&quantity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InventoryAdjustment{}, ErrListingNotFound
		}
		return InventoryAdjustment{}, fmt.Errorf("lock inventory: %w", err)
	}
	if quantity+delta < 0 {
		return InventoryAdjustment{}, ErrInsufficientStock
	}
	quantity += delta

	if _, err := tx.Exec(ctx, "UPDATE inventory SET quantity = $2, updated_at = now() WHERE listing_id = $1", listingID, quantity); err != nil {
		return InventoryAdjustment{}, fmt.Errorf("update inventory: %w", err)
	}

	adjustment := InventoryAdjustment{
		ID:        mustUUIDv7(),
		ListingID: listingID,
		ActorID:   actorID,
		Delta:     delta,
		Quantity:  quantity,
		Reason:    reason,
		CreatedAt: time.Now(),
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO inventory_adjustments (id, listing_id, actor_id, delta, quantity, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`, adjustment.ID, listingID, actorID, delta, quantity, reason).Scan(&adjustment.CreatedAt)
	if err != nil {
		return InventoryAdjustment{}, fmt.Errorf("insert inventory adjustment: %w", err)
	}
	return adjustment, nil
}

func (s *Store) setInventoryTx(ctx context.Context, tx pgx.Tx, listingID, actorID uuid.UUID, target int, reason string) error {
	if target < 0 {
		return ErrInsufficientStock
	}
	var status string
	if err := tx.QueryRow(ctx, "SELECT status FROM listings WHERE id = $1", listingID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrListingNotFound
		}
		return fmt.Errorf("select listing for inventory: %w", err)
	}
	if status == "archived" {
		return ErrInvalidState
	}

	var quantity int
	if err := tx.QueryRow(ctx, "SELECT quantity FROM inventory WHERE listing_id = $1 FOR UPDATE", listingID).Scan(&quantity); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrListingNotFound
		}
		return fmt.Errorf("lock inventory: %w", err)
	}
	delta := target - quantity
	if delta == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, "UPDATE inventory SET quantity = $2, updated_at = now() WHERE listing_id = $1", listingID, target); err != nil {
		return fmt.Errorf("update inventory: %w", err)
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO inventory_adjustments (id, listing_id, actor_id, delta, quantity, reason)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, mustUUIDv7(), listingID, actorID, delta, target, reason)
	if err != nil {
		return fmt.Errorf("insert inventory adjustment: %w", err)
	}
	return nil
}
