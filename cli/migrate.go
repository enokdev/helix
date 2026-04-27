package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/enokdev/helix/cli/internal/migrate"
)

// CreateMigrationOptions configures the helix db migrate create entry point.
type CreateMigrationOptions struct {
	Dir  string
	Name string
}

// MigrationOptions configures helix db migrate execution entry points.
type MigrationOptions struct {
	Dir         string
	DatabaseURL string
	Output      io.Writer
}

// CreateMigration creates a timestamped migration file.
func CreateMigration(ctx context.Context, opts CreateMigrationOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: db migrate create %s: nil context", opts.Name)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: db migrate create %s: %w", opts.Name, err)
	}
	if err := migrate.Create(ctx, migrate.CreateOptions{RootDir: opts.Dir, Name: opts.Name}); err != nil {
		return fmt.Errorf("cli: db migrate create %s: %w", opts.Name, err)
	}
	return nil
}

// MigrateUp applies pending migrations.
func MigrateUp(ctx context.Context, opts MigrationOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: db migrate up: nil context")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: db migrate up: %w", err)
	}
	if err := migrate.Up(ctx, migrate.Options{RootDir: opts.Dir, DatabaseURL: opts.DatabaseURL, Output: opts.Output}); err != nil {
		return fmt.Errorf("cli: db migrate up: %w", err)
	}
	return nil
}

// MigrateDown rolls back the latest applied migration.
func MigrateDown(ctx context.Context, opts MigrationOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: db migrate down: nil context")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: db migrate down: %w", err)
	}
	if err := migrate.Down(ctx, migrate.Options{RootDir: opts.Dir, DatabaseURL: opts.DatabaseURL, Output: opts.Output}); err != nil {
		return fmt.Errorf("cli: db migrate down: %w", err)
	}
	return nil
}

// MigrationStatus prints migration status.
func MigrationStatus(ctx context.Context, opts MigrationOptions) error {
	if ctx == nil {
		return fmt.Errorf("cli: db migrate status: nil context")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cli: db migrate status: %w", err)
	}
	if err := migrate.Status(ctx, migrate.Options{RootDir: opts.Dir, DatabaseURL: opts.DatabaseURL, Output: opts.Output}); err != nil {
		return fmt.Errorf("cli: db migrate status: %w", err)
	}
	return nil
}
