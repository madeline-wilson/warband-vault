package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
	"warband-vault/internal/config"
	"warband-vault/internal/migration"
	"warband-vault/internal/platform"
	"warband-vault/internal/validation"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("record not found")

type Store struct {
	db         *sql.DB
	logger     *slog.Logger
	Campaigns  *CampaignRepository
	Characters *CharacterRepository
}

func Open(ctx context.Context, paths config.Paths, logger *slog.Logger) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(paths.Database), 0o755); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}
	db, err := sql.Open("sqlite", paths.Database+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}
	if _, err := db.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if err := migration.Apply(ctx, db, paths.Database, paths.BackupsDir, logger); err != nil {
		db.Close()
		return nil, err
	}
	store := &Store{db: db, logger: logger}
	store.Campaigns = &CampaignRepository{store: store}
	store.Characters = &CharacterRepository{store: store}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) ImportCampaign(ctx context.Context, input *campaign.Campaign) (*campaign.Campaign, error) {
	if err := validation.ValidateCampaign(input); err != nil {
		return nil, err
	}
	copied := cloneCampaign(input)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback()
	if copied.ID == "" || idExists(ctx, tx, "campaigns", copied.ID) {
		id, err := platform.NewID()
		if err != nil {
			return nil, err
		}
		copied.ID = id
	}
	now := time.Now().UTC()
	if copied.CreatedAt.IsZero() {
		copied.CreatedAt = now
	}
	copied.UpdatedAt = now
	if err := insertCampaign(ctx, tx, &copied); err != nil {
		return nil, err
	}
	for i := range copied.Characters {
		ch := &copied.Characters[i]
		if ch.ID == "" || idExists(ctx, tx, "characters", ch.ID) {
			id, err := platform.NewID()
			if err != nil {
				return nil, err
			}
			ch.ID = id
		}
		ch.CampaignID = copied.ID
		if err := ensureCharacterDefaults(ch, now); err != nil {
			return nil, err
		}
		if err := insertCharacter(ctx, tx, ch); err != nil {
			return nil, err
		}
		if err := replaceCharacterDetails(ctx, tx, ch); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit import transaction: %w", err)
	}
	return &copied, nil
}

func cloneCampaign(input *campaign.Campaign) campaign.Campaign {
	copied := *input
	copied.Characters = append([]character.Character(nil), input.Characters...)
	for i := range copied.Characters {
		ch := &copied.Characters[i]
		ch.Equipment = append([]character.EquipmentItem(nil), ch.Equipment...)
		ch.Traits = append([]character.Trait(nil), ch.Traits...)
		ch.Injuries = append([]character.Injury(nil), ch.Injuries...)
		if ch.CustomFields != nil {
			fields := make(map[string]string, len(ch.CustomFields))
			for key, value := range ch.CustomFields {
				fields[key] = value
			}
			ch.CustomFields = fields
		}
	}
	return copied
}

func idExists(ctx context.Context, tx *sql.Tx, table, id string) bool {
	if id == "" {
		return false
	}
	var exists int
	query := fmt.Sprintf("SELECT 1 FROM %s WHERE id = ? LIMIT 1", table)
	err := tx.QueryRowContext(ctx, query, id).Scan(&exists)
	return err == nil
}

func withTx(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func nowIfZero(t time.Time, now time.Time) time.Time {
	if t.IsZero() {
		return now
	}
	return t.UTC()
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		t = time.Now().UTC()
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(value string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intBool(value int) bool {
	return value != 0
}
