package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrCategoryNotFound   = errors.New("category not found")
	ErrCategoryInUse      = errors.New("category in use")
	ErrSlugTaken          = errors.New("category slug taken")
	ErrCategoryParent     = errors.New("invalid category parent")
	ErrListingNotFound    = errors.New("listing not found")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidState       = errors.New("invalid listing state")
	ErrRequiresReview     = errors.New("listing requires review")
	ErrCategoryInactive   = errors.New("category inactive")
	ErrInsufficientStock  = errors.New("insufficient stock")
	ErrImageLimit         = errors.New("image limit reached")
	ErrImageNotFound      = errors.New("image not found")
	ErrUnsupportedImage   = errors.New("unsupported image type")
	ErrInvalidListingData = errors.New("invalid listing data")
)

type Category struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	ParentID  *uuid.UUID
	IsActive  bool
	CreatedAt time.Time
}

type CategoryInput struct {
	Name     string
	Slug     string
	ParentID *uuid.UUID
	IsActive bool
}

type ListingInput struct {
	Title        string
	Description  string
	CategoryID   uuid.UUID
	PriceCents   int64
	Currency     string
	Condition    string
	FreeShipping bool
	Stock        int
}

type ListingPatch struct {
	Title        *string
	Description  *string
	CategoryID   *uuid.UUID
	PriceCents   *int64
	Condition    *string
	FreeShipping *bool
	Stock        *int
}

type ListingImage struct {
	ID          uuid.UUID
	ObjectKey   string
	ContentType string
	ByteSize    int64
	Position    int
	CreatedAt   time.Time
	URL         string
}

type Listing struct {
	ID              uuid.UUID
	SellerID        uuid.UUID
	SellerName      string
	CategoryID      uuid.UUID
	CategoryName    string
	CategorySlug    string
	Title           string
	Description     string
	PriceCents      int64
	Currency        string
	Condition       string
	FreeShipping    bool
	Status          string
	SearchText      string
	RejectionReason *string
	Stock           int
	Images          []ListingImage
	SubmittedAt     *time.Time
	PublishedAt     *time.Time
	ArchivedAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (s *Store) ListPublicCategories(ctx context.Context) ([]Category, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, slug, parent_id, is_active, created_at
		FROM categories WHERE is_active ORDER BY name, id
	`)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()

	categories := make([]Category, 0)
	for rows.Next() {
		category, err := scanCategory(rows)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}
	return categories, rows.Err()
}

func (s *Store) CategoryByID(ctx context.Context, id uuid.UUID) (Category, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, slug, parent_id, is_active, created_at
		FROM categories WHERE id = $1
	`, id)
	category, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrCategoryNotFound
	}
	return category, err
}

func (s *Store) CreateCategory(ctx context.Context, input CategoryInput) (Category, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Category{}, fmt.Errorf("new category id: %w", err)
	}
	if input.ParentID != nil {
		if _, err := s.CategoryByID(ctx, *input.ParentID); err != nil {
			return Category{}, ErrCategoryParent
		}
	}

	row := s.pool.QueryRow(ctx, `
		INSERT INTO categories (id, name, slug, parent_id, is_active)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, slug, parent_id, is_active, created_at
	`, id, input.Name, input.Slug, input.ParentID, input.IsActive)
	category, err := scanCategory(row)
	if isUniqueViolation(err, "categories_slug_idx") {
		return Category{}, ErrSlugTaken
	}
	if err != nil {
		return Category{}, fmt.Errorf("insert category: %w", err)
	}
	return category, nil
}

func (s *Store) UpdateCategory(ctx context.Context, id uuid.UUID, input CategoryInput) (Category, error) {
	if input.ParentID != nil {
		if *input.ParentID == id {
			return Category{}, ErrCategoryParent
		}
		cycle, err := s.categoryCreatesCycle(ctx, id, *input.ParentID)
		if err != nil {
			return Category{}, err
		}
		if cycle {
			return Category{}, ErrCategoryParent
		}
	}

	row := s.pool.QueryRow(ctx, `
		UPDATE categories
		SET name = $2, slug = $3, parent_id = $4, is_active = $5, updated_at = now()
		WHERE id = $1
		RETURNING id, name, slug, parent_id, is_active, created_at
	`, id, input.Name, input.Slug, input.ParentID, input.IsActive)
	category, err := scanCategory(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return Category{}, ErrCategoryNotFound
	}
	if isUniqueViolation(err, "categories_slug_idx") {
		return Category{}, ErrSlugTaken
	}
	if err != nil {
		return Category{}, fmt.Errorf("update category: %w", err)
	}
	return category, nil
}

func (s *Store) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete category: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var listings int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM listings WHERE category_id = $1", id).Scan(&listings); err != nil {
		return fmt.Errorf("count category listings: %w", err)
	}
	if listings > 0 {
		return ErrCategoryInUse
	}
	var children int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM categories WHERE parent_id = $1", id).Scan(&children); err != nil {
		return fmt.Errorf("count category children: %w", err)
	}
	if children > 0 {
		return ErrCategoryInUse
	}
	result, err := tx.Exec(ctx, "DELETE FROM categories WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrCategoryNotFound
	}
	return tx.Commit(ctx)
}

func (s *Store) categoryCreatesCycle(ctx context.Context, id, parentID uuid.UUID) (bool, error) {
	var cycle bool
	err := s.pool.QueryRow(ctx, `
		WITH RECURSIVE ancestors AS (
			SELECT id, parent_id FROM categories WHERE id = $2
			UNION ALL
			SELECT c.id, c.parent_id FROM categories c JOIN ancestors a ON c.id = a.parent_id
		)
		SELECT EXISTS (SELECT 1 FROM ancestors WHERE id = $1)
	`, id, parentID).Scan(&cycle)
	if err != nil {
		return false, fmt.Errorf("check category cycle: %w", err)
	}
	return cycle, nil
}

type categoryScanner interface {
	Scan(dest ...any) error
}

func scanCategory(scanner categoryScanner) (Category, error) {
	var category Category
	err := scanner.Scan(&category.ID, &category.Name, &category.Slug, &category.ParentID, &category.IsActive, &category.CreatedAt)
	return category, err
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := true
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			builder.WriteRune(character)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
