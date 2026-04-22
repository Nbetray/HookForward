package repository

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

func OpenPostgres(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, dsn)
}

func RunMigrations(ctx context.Context, db *pgxpool.Pool, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return err
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		names = append(names, entry.Name())
	}

	sort.Strings(names)

	for _, name := range names {
		sqlBytes, readErr := os.ReadFile(filepath.Join(migrationsDir, name))
		if readErr != nil {
			return readErr
		}

		if _, execErr := db.Exec(ctx, string(sqlBytes)); execErr != nil {
			return execErr
		}
	}

	return nil
}
