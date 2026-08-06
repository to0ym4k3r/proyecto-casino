// Command migrate applies pending versioned SQL migrations to the database.
//
// It connects using the DATABASE_URL environment variable, falling back to the
// standard DB_* config, and applies every pending migrations/<NNN>_<name>.sql
// file in order. Completed versions are recorded in the schema_migrations
// table, so a later run only applies files that were added since.
package main

import (
	"context"
	"log"
	"os"
	"time"

	"casino-backend/internal/config"
	"casino-backend/internal/database"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	url := os.Getenv("DATABASE_URL")
	if url == "" {
		url = config.Load().Database.DSN()
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("parse database url: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}

	migrations, err := database.LoadMigrations("migrations")
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}

	applied, err := database.ApplyMigrations(ctx, pool, migrations)
	if err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	log.Printf("applied %d migration(s)", applied)
}
