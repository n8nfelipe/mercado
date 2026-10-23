package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrEmailExists       = errors.New("email already exists")
	ErrDocumentExists    = errors.New("document already exists")
	ErrSellerProfileGone = errors.New("seller profile not found")
)

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Roles     []string
	CreatedAt time.Time
}

type SellerProfile struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	DisplayName string
	Document    string
	Contact     string
	CreatedAt   time.Time
}

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) CreateUser(ctx context.Context, name, email, emailNormalized, passwordHash string) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, fmt.Errorf("new user id: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin user transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var user User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (id, name, email, email_normalized, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, name, email, created_at
	`, id, name, email, emailNormalized, passwordHash).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, "users_email_normalized_idx") {
			return User{}, ErrEmailExists
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO user_roles (user_id, role) VALUES ($1, 'buyer')", id); err != nil {
		return User{}, fmt.Errorf("insert buyer role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit user: %w", err)
	}
	user.Roles = []string{"buyer"}
	return user, nil
}

func (s *Store) UserByEmail(ctx context.Context, emailNormalized string) (User, string, error) {
	var user User
	var passwordHash string
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.email, u.password_hash, u.created_at,
		       COALESCE(array_agg(ur.role) FILTER (WHERE ur.role IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.email_normalized = $1
		GROUP BY u.id
	`, emailNormalized).Scan(&user.ID, &user.Name, &user.Email, &passwordHash, &user.CreatedAt, &user.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", ErrNotFound
	}
	if err != nil {
		return User{}, "", fmt.Errorf("select user by email: %w", err)
	}
	return user, passwordHash, nil
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.email, u.created_at,
		       COALESCE(array_agg(ur.role) FILTER (WHERE ur.role IS NOT NULL), '{}')
		FROM users u
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE u.id = $1
		GROUP BY u.id
	`, id).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select user by id: %w", err)
	}
	return user, nil
}

func (s *Store) AddRole(ctx context.Context, userID uuid.UUID, role string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_roles (user_id, role) VALUES ($1, $2)
		ON CONFLICT (user_id, role) DO NOTHING
	`, userID, role)
	if err != nil {
		return fmt.Errorf("add role: %w", err)
	}
	return nil
}

func (s *Store) CreateSession(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("new session id: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, id, userID, tokenHash, expiresAt)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert session: %w", err)
	}
	return id, nil
}

func (s *Store) UserBySession(ctx context.Context, tokenHash []byte) (User, error) {
	var user User
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.name, u.email, u.created_at,
		       COALESCE(array_agg(ur.role) FILTER (WHERE ur.role IS NOT NULL), '{}')
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN user_roles ur ON ur.user_id = u.id
		WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now()
		GROUP BY u.id
	`, tokenHash).Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt, &user.Roles)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("select session user: %w", err)
	}
	return user, nil
}

func (s *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE sessions SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL
	`, tokenHash)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *Store) CreateSellerProfile(ctx context.Context, userID uuid.UUID, displayName, document, documentNormalized, contact string) (SellerProfile, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return SellerProfile{}, fmt.Errorf("new seller id: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SellerProfile{}, fmt.Errorf("begin seller transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var profile SellerProfile
	err = tx.QueryRow(ctx, `
		INSERT INTO seller_profiles (id, user_id, display_name, document, document_normalized, contact)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, display_name, document, contact, created_at
	`, id, userID, displayName, document, documentNormalized, contact).
		Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Document, &profile.Contact, &profile.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, "seller_profiles_document_normalized_idx") {
			return SellerProfile{}, ErrDocumentExists
		}
		if isUniqueViolation(err, "seller_profiles_user_id_key") {
			return SellerProfile{}, ErrDocumentExists
		}
		return SellerProfile{}, fmt.Errorf("insert seller profile: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role) VALUES ($1, 'seller')
		ON CONFLICT (user_id, role) DO NOTHING
	`, userID); err != nil {
		return SellerProfile{}, fmt.Errorf("insert seller role: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return SellerProfile{}, fmt.Errorf("commit seller profile: %w", err)
	}
	return profile, nil
}

func (s *Store) SellerByUserID(ctx context.Context, userID uuid.UUID) (SellerProfile, error) {
	var profile SellerProfile
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, display_name, document, contact, created_at
		FROM seller_profiles WHERE user_id = $1
	`, userID).Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Document, &profile.Contact, &profile.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SellerProfile{}, ErrNotFound
	}
	if err != nil {
		return SellerProfile{}, fmt.Errorf("select seller by user: %w", err)
	}
	return profile, nil
}

func (s *Store) SellerByID(ctx context.Context, id uuid.UUID) (SellerProfile, error) {
	var profile SellerProfile
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, display_name, document, contact, created_at
		FROM seller_profiles WHERE id = $1
	`, id).Scan(&profile.ID, &profile.UserID, &profile.DisplayName, &profile.Document, &profile.Contact, &profile.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return SellerProfile{}, ErrNotFound
	}
	if err != nil {
		return SellerProfile{}, fmt.Errorf("select seller by id: %w", err)
	}
	return profile, nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}
