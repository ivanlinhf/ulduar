package dbmigrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	migrationfiles "github.com/ivanlin/ulduar/apps/backend/db/migrations"
	"github.com/jackc/pgx/v5"
)

const schemaMigrationsTable = "schema_migrations"

type Runner struct {
	conn       *pgx.Conn
	migrations []Migration
}

type Migration struct {
	Version  string
	Name     string
	UpFile   string
	DownFile string
}

func New(conn *pgx.Conn) (*Runner, error) {
	migrations, err := loadMigrations(migrationfiles.Files)
	if err != nil {
		return nil, err
	}

	return &Runner{
		conn:       conn,
		migrations: migrations,
	}, nil
}

func (r *Runner) Up(ctx context.Context) error {
	if err := r.ensureMigrationsTable(ctx); err != nil {
		return err
	}

	appliedVersions, err := r.appliedVersions(ctx)
	if err != nil {
		return err
	}

	for _, migration := range r.migrations {
		if appliedVersions[migration.Version] {
			continue
		}

		if err := r.applyUp(ctx, migration); err != nil {
			return err
		}
	}

	return nil
}

func (r *Runner) Down(ctx context.Context) error {
	if err := r.ensureMigrationsTable(ctx); err != nil {
		return err
	}

	version, err := r.latestAppliedVersion(ctx)
	if err != nil {
		return err
	}

	if version == "" {
		return nil
	}

	migration, err := r.findMigration(version)
	if err != nil {
		return err
	}

	return r.applyDown(ctx, migration)
}

func (r *Runner) ensureMigrationsTable(ctx context.Context) error {
	_, err := r.conn.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	if err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	return nil
}

func (r *Runner) appliedVersions(ctx context.Context) (map[string]bool, error) {
	rows, err := r.conn.Query(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate applied migrations: %w", rows.Err())
	}

	return applied, nil
}

func (r *Runner) latestAppliedVersion(ctx context.Context) (string, error) {
	var version string
	err := r.conn.QueryRow(
		ctx,
		`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`,
	).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query latest applied migration: %w", err)
	}

	return version, nil
}

func (r *Runner) applyUp(ctx context.Context, migration Migration) error {
	sqlBytes, err := fs.ReadFile(migrationfiles.Files, migration.UpFile)
	if err != nil {
		return fmt.Errorf("read up migration %s: %w", migration.UpFile, err)
	}

	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin up migration %s: %w", migration.Version, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("execute up migration %s: %w", migration.Version, err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO schema_migrations (version, name) VALUES ($1, $2)`,
		migration.Version,
		migration.Name,
	); err != nil {
		return fmt.Errorf("record up migration %s: %w", migration.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit up migration %s: %w", migration.Version, err)
	}

	return nil
}

func (r *Runner) applyDown(ctx context.Context, migration Migration) error {
	sqlBytes, err := fs.ReadFile(migrationfiles.Files, migration.DownFile)
	if err != nil {
		return fmt.Errorf("read down migration %s: %w", migration.DownFile, err)
	}

	tx, err := r.conn.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin down migration %s: %w", migration.Version, err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
		return fmt.Errorf("execute down migration %s: %w", migration.Version, err)
	}

	if _, err := tx.Exec(
		ctx,
		`DELETE FROM schema_migrations WHERE version = $1`,
		migration.Version,
	); err != nil {
		return fmt.Errorf("delete migration record %s: %w", migration.Version, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit down migration %s: %w", migration.Version, err)
	}

	return nil
}

func (r *Runner) findMigration(version string) (Migration, error) {
	for _, migration := range r.migrations {
		if migration.Version == version {
			return migration, nil
		}
	}

	return Migration{}, fmt.Errorf("migration version %s not found", version)
}

func loadMigrations(files fs.FS) ([]Migration, error) {
	entries, err := fs.Glob(files, "*.up.sql")
	if err != nil {
		return nil, fmt.Errorf("glob up migrations: %w", err)
	}

	migrations := make([]Migration, 0, len(entries))
	for _, upFile := range entries {
		version, name, err := parseMigrationFilename(upFile, ".up.sql")
		if err != nil {
			return nil, err
		}

		downFile := strings.TrimSuffix(upFile, ".up.sql") + ".down.sql"
		if _, err := fs.Stat(files, downFile); err != nil {
			return nil, fmt.Errorf("missing down migration for %s", upFile)
		}

		migrations = append(migrations, Migration{
			Version:  version,
			Name:     name,
			UpFile:   upFile,
			DownFile: downFile,
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func parseMigrationFilename(filename, suffix string) (string, string, error) {
	base := strings.TrimSuffix(filename, suffix)
	parts := strings.SplitN(base, "_", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid migration filename: %s", filename)
	}

	return parts[0], parts[1], nil
}
