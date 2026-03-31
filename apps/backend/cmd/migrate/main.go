package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ivanlin/ulduar/apps/backend/internal/dbmigrate"
	"github.com/jackc/pgx/v5"
)

func main() {
	direction, err := parseDirection(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL must not be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	connConfig, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		log.Fatalf("parse database url: %v", err)
	}
	connConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer conn.Close(context.Background())

	runner, err := dbmigrate.New(conn)
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}

	switch direction {
	case "up":
		if err := runner.Up(ctx); err != nil {
			log.Fatalf("apply migrations up: %v", err)
		}
		log.Print("migrations applied")
	case "down":
		if err := runner.Down(ctx); err != nil {
			log.Fatalf("apply migration down: %v", err)
		}
		log.Print("migration rolled back")
	}
}

func parseDirection(args []string) (string, error) {
	if len(args) == 0 {
		return "up", nil
	}

	direction := strings.TrimSpace(strings.ToLower(args[0]))
	switch direction {
	case "up", "down":
		return direction, nil
	default:
		return "", fmt.Errorf("unsupported migration direction %q; use up or down", direction)
	}
}
