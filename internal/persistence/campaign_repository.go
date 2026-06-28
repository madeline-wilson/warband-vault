package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"warband-vault/internal/campaign"
	"warband-vault/internal/platform"
	"warband-vault/internal/validation"
)

type CampaignRepository struct {
	store *Store
}

func (r *CampaignRepository) Create(ctx context.Context, c *campaign.Campaign) error {
	if err := validation.ValidateCampaign(c); err != nil {
		return err
	}
	now := time.Now().UTC()
	if c.ID == "" {
		id, err := platform.NewID()
		if err != nil {
			return err
		}
		c.ID = id
	}
	c.CreatedAt = nowIfZero(c.CreatedAt, now)
	c.UpdatedAt = now
	return withTx(ctx, r.store.db, func(tx *sql.Tx) error {
		if err := insertCampaign(ctx, tx, c); err != nil {
			return err
		}
		for i := range c.Characters {
			ch := &c.Characters[i]
			ch.CampaignID = c.ID
			if err := ensureCharacterDefaults(ch, now); err != nil {
				return err
			}
			if err := insertCharacter(ctx, tx, ch); err != nil {
				return err
			}
			if err := replaceCharacterDetails(ctx, tx, ch); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *CampaignRepository) Update(ctx context.Context, c *campaign.Campaign) error {
	if err := validation.ValidateCampaign(c); err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC()
	result, err := r.store.db.ExecContext(ctx, `
		UPDATE campaigns
		SET name = ?, system_name = ?, description = ?, treasury = ?, archived = ?, updated_at = ?
		WHERE id = ?`,
		c.Name, c.SystemName, c.Description, c.Treasury, boolInt(c.Archived), formatTime(c.UpdatedAt), c.ID,
	)
	if err != nil {
		return fmt.Errorf("update campaign %s: %w", c.ID, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read campaign update count: %w", err)
	}
	if rows == 0 {
		return campaign.ErrNotFound
	}
	return nil
}

func (r *CampaignRepository) Delete(ctx context.Context, id string) error {
	result, err := r.store.db.ExecContext(ctx, `DELETE FROM campaigns WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete campaign %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read campaign delete count: %w", err)
	}
	if rows == 0 {
		return campaign.ErrNotFound
	}
	return nil
}

func (r *CampaignRepository) FindByID(ctx context.Context, id string) (*campaign.Campaign, error) {
	c, err := selectCampaign(ctx, r.store.db, id)
	if err != nil {
		return nil, err
	}
	characters, err := r.store.Characters.ListByCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	c.Characters = characters
	return c, nil
}

func (r *CampaignRepository) List(ctx context.Context, includeArchived bool) ([]campaign.Campaign, error) {
	query := `SELECT id, name, system_name, description, treasury, archived, created_at, updated_at FROM campaigns`
	args := []any{}
	if !includeArchived {
		query += ` WHERE archived = ?`
		args = append(args, 0)
	}
	query += ` ORDER BY archived ASC, updated_at DESC, name COLLATE NOCASE ASC`
	rows, err := r.store.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()
	var campaigns []campaign.Campaign
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		campaigns = append(campaigns, *c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate campaigns: %w", err)
	}
	return campaigns, nil
}

type campaignScanner interface {
	Scan(dest ...any) error
}

func insertCampaign(ctx context.Context, exec sqlExecer, c *campaign.Campaign) error {
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO campaigns(id, name, system_name, description, treasury, archived, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.SystemName, c.Description, c.Treasury, boolInt(c.Archived), formatTime(c.CreatedAt), formatTime(c.UpdatedAt),
	); err != nil {
		return fmt.Errorf("insert campaign %s: %w", c.ID, err)
	}
	return nil
}

func selectCampaign(ctx context.Context, queryer sqlQueryer, id string) (*campaign.Campaign, error) {
	row := queryer.QueryRowContext(ctx, `
		SELECT id, name, system_name, description, treasury, archived, created_at, updated_at
		FROM campaigns
		WHERE id = ?`, id)
	c, err := scanCampaign(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, campaign.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return c, nil
}

func scanCampaign(scanner campaignScanner) (*campaign.Campaign, error) {
	var c campaign.Campaign
	var archived int
	var createdAt, updatedAt string
	if err := scanner.Scan(&c.ID, &c.Name, &c.SystemName, &c.Description, &c.Treasury, &archived, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan campaign: %w", err)
	}
	c.Archived = intBool(archived)
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	return &c, nil
}
