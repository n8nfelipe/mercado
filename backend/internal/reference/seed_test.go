package reference

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"mercado/migrations"
)

type snapshot struct {
	Currencies []string
	Countries  []string
	Locales    []string
}

func TestMigrationsSeedAndRollback(t *testing.T) {
	if !dockerAvailable() {
		t.Skip("Docker is not available")
	}

	ctx := context.Background()
	dsn, container := startPostgres(t, ctx)
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("set dialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	first, err := Seed(ctx, pool, 1)
	if err != nil {
		t.Fatalf("seed 1: %v", err)
	}
	if first.Currencies != int64(len(currencies)) || first.Countries != int64(len(countries)) {
		t.Fatalf("unexpected reference counts: %+v", first)
	}
	if first.Locales <= int64(len(countries)) {
		t.Fatalf("seed did not add extra locales: %+v", first)
	}

	before, err := readSnapshot(ctx, pool)
	if err != nil {
		t.Fatalf("snapshot after first seed: %v", err)
	}
	again, err := Seed(ctx, pool, 1)
	if err != nil {
		t.Fatalf("seed 1 again: %v", err)
	}
	if !reflect.DeepEqual(first, again) {
		t.Fatalf("same seed is not idempotent: first=%+v again=%+v", first, again)
	}
	after, err := readSnapshot(ctx, pool)
	if err != nil {
		t.Fatalf("snapshot after second seed: %v", err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("same seed changed rows:\nbefore=%+v\nafter=%+v", before, after)
	}

	different, err := Seed(ctx, pool, 2)
	if err != nil {
		t.Fatalf("seed 2: %v", err)
	}
	if different.Fingerprint == first.Fingerprint {
		t.Fatalf("different seeds produced the same fingerprint")
	}
	if different.Locales != first.Locales+1 {
		t.Fatalf("unexpected locale count for seed 2: first=%d different=%d", first.Locales, different.Locales)
	}
	other, err := readSnapshot(ctx, pool)
	if err != nil {
		t.Fatalf("snapshot after different seed: %v", err)
	}
	if reflect.DeepEqual(before.Locales, other.Locales) {
		t.Fatalf("different seeds produced the same locales: %v", other.Locales)
	}
	if !reflect.DeepEqual(before.Currencies, other.Currencies) || !reflect.DeepEqual(before.Countries, other.Countries) {
		t.Fatalf("different seeds changed canonical currencies/countries")
	}

	if err := goose.DownTo(db, ".", 1); err != nil {
		t.Fatalf("rollback reference migration: %v", err)
	}
	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		t.Fatalf("read version after rollback: %v", err)
	}
	if version != 1 {
		t.Fatalf("expected version 1 after rollback, got %d", version)
	}
	column, err := hasColumn(ctx, pool, "locales", "is_extra")
	if err != nil {
		t.Fatalf("inspect rolled back column: %v", err)
	}
	if column {
		t.Fatalf("is_extra still exists after rollback")
	}
	preserved, err := countRows(ctx, pool, "locales")
	if err != nil {
		t.Fatalf("count preserved locales: %v", err)
	}
	if preserved != different.Locales {
		t.Fatalf("rollback lost rows: got %d want %d", preserved, different.Locales)
	}

	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
	restored, err := Seed(ctx, pool, 1)
	if err != nil {
		t.Fatalf("seed after reapply: %v", err)
	}
	if restored.Fingerprint != first.Fingerprint {
		t.Fatalf("seed after reapply changed fingerprint: %s != %s", restored.Fingerprint, first.Fingerprint)
	}
	restoredSnapshot, err := readSnapshot(ctx, pool)
	if err != nil {
		t.Fatalf("snapshot after reapply: %v", err)
	}
	if !reflect.DeepEqual(restoredSnapshot, before) {
		t.Fatalf("seed after reapply did not restore the same rows")
	}

	if err := goose.Reset(db, "."); err != nil {
		t.Fatalf("reset migrations: %v", err)
	}
	var empty bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public.locales') IS NULL`).Scan(&empty); err != nil {
		t.Fatalf("inspect schema after reset: %v", err)
	}
	if !empty {
		t.Fatalf("locales table still exists after reset")
	}
}

func readSnapshot(ctx context.Context, pool *pgxpool.Pool) (snapshot, error) {
	currencies, err := readCodes(ctx, pool, "SELECT code FROM currencies ORDER BY code")
	if err != nil {
		return snapshot{}, err
	}
	countries, err := readCodes(ctx, pool, "SELECT code FROM countries ORDER BY code")
	if err != nil {
		return snapshot{}, err
	}
	locales, err := readCodes(ctx, pool, "SELECT code FROM locales ORDER BY code")
	if err != nil {
		return snapshot{}, err
	}
	return snapshot{Currencies: currencies, Countries: countries, Locales: locales}, nil
}

func readCodes(ctx context.Context, pool *pgxpool.Pool, query string) ([]string, error) {
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		codes = append(codes, code)
	}
	return codes, rows.Err()
}

func countRows(ctx context.Context, pool *pgxpool.Pool, table string) (int64, error) {
	var total int64
	err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&total)
	return total, err
}

func hasColumn(ctx context.Context, pool *pgxpool.Pool, table, column string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)
	`, table, column).Scan(&exists)
	return exists, err
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
