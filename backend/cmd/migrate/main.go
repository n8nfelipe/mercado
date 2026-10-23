package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/pressly/goose/v3"

	"mercado/migrations"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	direction := flag.String("direction", "up", "up, down, reset or status")
	timeout := flag.Duration("timeout", 60*time.Second, "database connection timeout")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fail("DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fail("open database: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		fail("ping database: %v", err)
	}

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		fail("set goose dialect: %v", err)
	}

	switch *direction {
	case "up":
		err = goose.Up(db, ".")
	case "down":
		err = goose.Down(db, ".")
	case "reset":
		err = goose.Reset(db, ".")
	case "status":
		err = goose.Status(db, ".")
	default:
		fail("unknown direction %q: use up, down, reset or status", *direction)
	}
	if err != nil {
		fail("%s: %v", *direction, err)
	}

	version, err := goose.GetDBVersionContext(ctx, db)
	if err != nil {
		fail("read schema version: %v", err)
	}
	fmt.Printf("schema_version=%d\n", version)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "migrate: "+format+"\n", args...)
	os.Exit(1)
}
