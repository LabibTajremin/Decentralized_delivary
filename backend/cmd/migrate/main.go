// Command migrate applies schema migrations and demo data.
//
// Usage:
//
//	migrate up          apply every migration the database is missing
//	migrate down [n]    roll back the newest n migrations (default 1, 0 = all)
//	migrate status      show which migrations are applied, pending or dirty
//	migrate seed        load demo data (refused when APP_ENV=production)
//
// Configuration comes from the environment, the same as the API server, so a
// deployment configures one set of variables rather than two. See .env.example.
//
// This file is wiring only: argument parsing, a database connection and a call
// into internal/platform/migrate, which is where every decision lives and where
// the tests are. See docs/decisions/0006-migration-tooling.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/jackc/pgx/v5"

	"github.com/rootlogic-lab/delivery/backend/internal/platform/migrate"
	"github.com/rootlogic-lab/delivery/backend/internal/shared/config"
	"github.com/rootlogic-lab/delivery/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: migrate up|down [n]|status|seed")
	}

	cfg, err := config.Load(nil)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	conn, err := pgx.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()

	db := migrate.NewPgxExecer(conn)
	loaded, err := migrations.Load()
	if err != nil {
		return err
	}

	switch os.Args[1] {
	case "up":
		res, upErr := migrate.Up(ctx, db, loaded)
		fmt.Println(migrate.Describe(res))
		return upErr

	case "down":
		steps := 1
		if len(os.Args) > 2 {
			steps, err = strconv.Atoi(os.Args[2])
			if err != nil {
				return fmt.Errorf("down: %q is not a number of steps", os.Args[2])
			}
		}
		res, downErr := migrate.Down(ctx, db, loaded, steps)
		fmt.Printf("migrate: rolled back %d migration(s)\n", len(res.Applied))
		return downErr

	case "status":
		statuses, statusErr := migrate.Status(ctx, db, loaded)
		if statusErr != nil {
			return statusErr
		}
		fmt.Println(migrate.DescribeStatus(statuses))
		return nil

	case "seed":
		scripts, seedErr := migrations.Seeds()
		if seedErr != nil {
			return seedErr
		}
		ran, seedErr := migrate.Seed(ctx, db, cfg.IsProduction(), scripts)
		for _, name := range ran {
			fmt.Printf("migrate: seeded %s\n", name)
		}
		return seedErr

	default:
		return fmt.Errorf("unknown command %q: use up, down, status or seed", os.Args[1])
	}
}
