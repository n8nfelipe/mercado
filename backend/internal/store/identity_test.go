package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"mercado/internal/auth"
	"mercado/internal/store"
	"mercado/migrations"
)

func TestIdentityRepository(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available")
	}

	ctx := context.Background()
	dsn, container := startPostgres(t, ctx)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	repository := store.New(pool)

	hash, err := auth.HashPassword("senha-super-segura")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user, err := repository.CreateUser(ctx, "Ana", "Ana@Example.com", "ana@example.com", hash)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "buyer" {
		t.Fatalf("unexpected roles: %v", user.Roles)
	}

	if _, err := repository.CreateUser(ctx, "Outra", "outra@example.com", "ana@example.com", hash); !errors.Is(err, store.ErrEmailExists) {
		t.Fatalf("expected duplicate email error, got %v", err)
	}

	found, storedHash, err := repository.UserByEmail(ctx, "ana@example.com")
	if err != nil {
		t.Fatalf("select user: %v", err)
	}
	if found.ID != user.ID || storedHash != hash {
		t.Fatal("selected user does not match")
	}
	ok, err := auth.VerifyPassword("senha-super-segura", storedHash)
	if err != nil || !ok {
		t.Fatalf("stored hash does not verify: ok=%v err=%v", ok, err)
	}

	token, tokenHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new session token: %v", err)
	}
	if len(token) == 0 {
		t.Fatal("empty token")
	}
	if _, err := repository.CreateSession(ctx, user.ID, tokenHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session: %v", err)
	}
	sessionUser, err := repository.UserBySession(ctx, tokenHash)
	if err != nil {
		t.Fatalf("select session user: %v", err)
	}
	if sessionUser.ID != user.ID {
		t.Fatal("session user does not match")
	}

	if err := repository.RevokeSession(ctx, tokenHash); err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if _, err := repository.UserBySession(ctx, tokenHash); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked session was accepted: %v", err)
	}

	secondToken, secondHash, err := auth.NewSessionToken()
	if err != nil {
		t.Fatalf("new second token: %v", err)
	}
	if secondToken == token {
		t.Fatal("session tokens should rotate")
	}
	if _, err := repository.CreateSession(ctx, user.ID, secondHash, time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create session to expire: %v", err)
	}
	if _, err := db.Exec("UPDATE sessions SET created_at = now() - interval '2 hours', expires_at = now() - interval '1 hour' WHERE token_hash = $1", secondHash); err != nil {
		t.Fatalf("expire session: %v", err)
	}
	if _, err := repository.UserBySession(ctx, secondHash); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired session was accepted: %v", err)
	}

	profile, err := repository.CreateSellerProfile(ctx, user.ID, "Loja da Ana", "12345678901", "12345678901", "ana@example.com")
	if err != nil {
		t.Fatalf("create seller profile: %v", err)
	}
	if profile.Document != "12345678901" {
		t.Fatal("seller document not stored")
	}

	other, err := repository.CreateUser(ctx, "Bia", "bia@example.com", "bia@example.com", hash)
	if err != nil {
		t.Fatalf("create other user: %v", err)
	}
	if _, err := repository.CreateSellerProfile(ctx, other.ID, "Loja da Bia", "12345678901", "12345678901", "bia@example.com"); !errors.Is(err, store.ErrDocumentExists) {
		t.Fatalf("expected duplicate document error, got %v", err)
	}

	roles, err := repository.UserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("select user by id: %v", err)
	}
	if !contains(roles.Roles, "seller") {
		t.Fatalf("seller role was not added: %v", roles.Roles)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
