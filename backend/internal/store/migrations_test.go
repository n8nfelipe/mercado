package store_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"mercado/migrations"
)

func TestIdentityAndCatalogMigrations(t *testing.T) {
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

	for _, table := range []string{
		"users", "user_roles", "sessions", "seller_profiles",
		"categories", "listings", "listing_images", "inventory",
		"inventory_adjustments", "listing_moderation_events",
	} {
		if !tableExists(t, db, table) {
			t.Fatalf("table %s was not created", table)
		}
	}

	for _, index := range []string{
		"users_email_normalized_idx",
		"seller_profiles_document_normalized_idx",
		"categories_slug_idx",
		"listings_search_text_trgm_idx",
		"listings_published_idx",
	} {
		if !indexExists(t, db, index) {
			t.Fatalf("index %s was not created", index)
		}
	}

	insertUser(t, db, "00000000-0000-7000-8000-000000000001", "a@example.com")
	if _, err := db.Exec(`
		INSERT INTO users (id, name, email, email_normalized, password_hash)
		VALUES ('00000000-0000-7000-8000-000000000002', 'Other', 'b@example.com', 'a@example.com', 'hash')
	`); err == nil {
		t.Fatal("duplicate normalized email was accepted")
	}

	if err := goose.DownTo(db, ".", 3); err != nil {
		t.Fatalf("rollback catalog migration: %v", err)
	}
	if tableExists(t, db, "listings") || tableExists(t, db, "categories") {
		t.Fatal("catalog tables still exist after down")
	}
	if !tableExists(t, db, "users") {
		t.Fatal("identity tables were removed by catalog down")
	}

	if err := goose.DownTo(db, ".", 2); err != nil {
		t.Fatalf("rollback identity migration: %v", err)
	}
	if tableExists(t, db, "users") || tableExists(t, db, "sessions") {
		t.Fatal("identity tables still exist after down")
	}
}

func startPostgres(t *testing.T, ctx context.Context) (string, testcontainers.Container) {
	t.Helper()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "mercado",
				"POSTGRES_PASSWORD": "mercado",
				"POSTGRES_DB":       "mercado",
			},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5432/tcp").WithStartupTimeout(90*time.Second),
				wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(90*time.Second),
			),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	return fmt.Sprintf("postgres://mercado:mercado@%s:%s/mercado?sslmode=disable", host, port.Port()), container
}

func dockerAvailable() bool {
	if os.Getenv("DOCKER_HOST") != "" {
		return true
	}
	_, err := os.Stat("/var/run/docker.sock")
	return err == nil
}

func tableExists(t *testing.T, db *sql.DB, table string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return exists
}

func indexExists(t *testing.T, db *sql.DB, index string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow("SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = $1)", index).Scan(&exists); err != nil {
		t.Fatalf("check index %s: %v", index, err)
	}
	return exists
}

func insertUser(t *testing.T, db *sql.DB, id, email string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO users (id, name, email, email_normalized, password_hash)
		VALUES ($1, 'User', $2, lower($2), 'hash')
	`, id, email); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}
