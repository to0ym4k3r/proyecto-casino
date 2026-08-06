// Package database provides versioned SQL schema migration support.
package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// schemaMigrationsDDL creates the table that records every applied version.
// It is intentionally created with IF NOT EXISTS so the runner is idempotent.
const schemaMigrationsDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`

// Migration is a single versioned SQL migration file.
type Migration struct {
	Version string // zero-padded sequence number, e.g. "001"
	Name    string // human-readable slug, e.g. "init"
	Path    string // original filename
	SQL     string // full file contents
}

// ParseMigrationFilename extracts the sequence number and slug from a
// migration filename such as "001_init.sql". It returns ok=false for any
// filename that does not follow the <NNN>_<name>.sql convention.
func ParseMigrationFilename(name string) (version, slug string, ok bool) {
	if !strings.HasSuffix(name, ".sql") {
		return "", "", false
	}

	base := strings.TrimSuffix(name, ".sql")
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return "", "", false
	}

	return parts[0], parts[1], true
}

// LoadMigrations reads every versioned *.sql file in dir and returns them
// ordered by sequence number. A *.sql file that does not follow the
// <NNN>_<name>.sql convention fails loudly instead of being silently skipped.
func LoadMigrations(dir string) ([]Migration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}

	var migrations []Migration
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, slug, ok := ParseMigrationFilename(entry.Name())
		if !ok {
			return nil, fmt.Errorf("migration file %q does not match <NNN>_<name>.sql", entry.Name())
		}

		body, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("read migration %q: %w", entry.Name(), err)
		}

		migrations = append(migrations, Migration{
			Version: version,
			Name:    slug,
			Path:    entry.Name(),
			SQL:     string(body),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return atoi(migrations[i].Version) < atoi(migrations[j].Version)
	})

	return migrations, nil
}

// ApplyMigrations applies every pending migration in sequence order, recording
// each completed version in the schema_migrations table. Each migration runs
// inside its own transaction so a failure leaves the schema unchanged for that
// migration. It returns the number of migrations applied.
func ApplyMigrations(ctx context.Context, pool *pgxpool.Pool, migrations []Migration) (int, error) {
	if _, err := pool.Exec(ctx, schemaMigrationsDDL); err != nil {
		return 0, fmt.Errorf("ensure schema_migrations table: %w", err)
	}

	applied, err := appliedVersions(ctx, pool)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, migration := range migrations {
		if _, done := applied[migration.Version]; done {
			continue
		}
		if err := applyOne(ctx, pool, migration); err != nil {
			return count, fmt.Errorf("apply migration %s (%s): %w", migration.Version, migration.Name, err)
		}
		count++
	}

	return count, nil
}

// appliedVersions returns the set of versions already recorded in
// schema_migrations.
func appliedVersions(ctx context.Context, pool *pgxpool.Pool) (map[string]struct{}, error) {
	rows, err := pool.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]struct{})
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", err)
	}

	return applied, nil
}

// applyOne runs a single migration and its version record inside one
// transaction. The SQL is executed with no arguments, so pgx uses the simple
// query protocol and multi-statement files are supported.
func applyOne(ctx context.Context, pool *pgxpool.Pool, migration Migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, migration.SQL); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, migration.Version); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// atoi converts a migration version to its numeric value for sorting.
func atoi(version string) int {
	n, _ := strconv.Atoi(version)
	return n
}
