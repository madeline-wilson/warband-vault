package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	embedded "warband-vault/migrations"
)

func Apply(ctx context.Context, db *sql.DB, dbPath, backupDir string, logger *slog.Logger) error {
	if db == nil {
		return fmt.Errorf("apply migrations: database is nil")
	}
	names, err := fs.Glob(embedded.Files, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("no embedded migrations found")
	}
	if err := backupDatabase(dbPath, backupDir); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration transaction: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}
	applied := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read applied migrations: %w", err)
	}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration: %w", err)
		}
		applied[version] = true
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close applied migrations rows: %w", err)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate applied migrations: %w", err)
	}
	for _, name := range names {
		version := strings.TrimSuffix(filepath.Base(name), ".sql")
		if applied[version] {
			continue
		}
		sqlBytes, err := embedded.Files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version, applied_at) VALUES (?, ?)`, version, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
			return fmt.Errorf("record migration %s: %w", version, err)
		}
		if logger != nil {
			logger.Info("database migration applied", "version", version)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrations: %w", err)
	}
	return nil
}

func backupDatabase(dbPath, backupDir string) error {
	info, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat database before backup: %w", err)
	}
	if info.Size() == 0 {
		return nil
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	src, err := os.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open database for backup: %w", err)
	}
	defer src.Close()
	dest := filepath.Join(backupDir, "warband-vault-"+time.Now().UTC().Format("20060102T150405Z")+".db")
	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create database backup: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy database backup: %w", err)
	}
	return nil
}
