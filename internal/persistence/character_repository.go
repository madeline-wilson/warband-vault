package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
	"warband-vault/internal/platform"
	"warband-vault/internal/validation"
)

type CharacterRepository struct {
	store *Store
}

func (r *CharacterRepository) Create(ctx context.Context, c *character.Character) error {
	if err := validation.ValidateCharacter(c); err != nil {
		return err
	}
	if c.CampaignID == "" {
		return fmt.Errorf("%w: campaign id is required", validation.ErrInvalid)
	}
	now := time.Now().UTC()
	if err := ensureCharacterDefaults(c, now); err != nil {
		return err
	}
	return withTx(ctx, r.store.db, func(tx *sql.Tx) error {
		if err := insertCharacter(ctx, tx, c); err != nil {
			return err
		}
		return replaceCharacterDetails(ctx, tx, c)
	})
}

func (r *CharacterRepository) Update(ctx context.Context, c *character.Character) error {
	if err := validation.ValidateCharacter(c); err != nil {
		return err
	}
	c.UpdatedAt = time.Now().UTC()
	return withTx(ctx, r.store.db, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `
			UPDATE characters
			SET name = ?, role = ?, level = ?, experience = ?, health = ?, movement = ?, armor = ?, notes = ?, updated_at = ?
			WHERE id = ?`,
			c.Name, c.Role, c.Level, c.Experience, c.Health, c.Movement, c.Armor, c.Notes, formatTime(c.UpdatedAt), c.ID,
		)
		if err != nil {
			return fmt.Errorf("update character %s: %w", c.ID, err)
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read character update count: %w", err)
		}
		if rows == 0 {
			return campaign.ErrNotFound
		}
		return replaceCharacterDetails(ctx, tx, c)
	})
}

func (r *CharacterRepository) Delete(ctx context.Context, id string) error {
	result, err := r.store.db.ExecContext(ctx, `DELETE FROM characters WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete character %s: %w", id, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read character delete count: %w", err)
	}
	if rows == 0 {
		return campaign.ErrNotFound
	}
	return nil
}

func (r *CharacterRepository) FindByID(ctx context.Context, id string) (*character.Character, error) {
	ch, err := selectCharacter(ctx, r.store.db, id)
	if err != nil {
		return nil, err
	}
	if err := loadCharacterDetails(ctx, r.store.db, ch); err != nil {
		return nil, err
	}
	return ch, nil
}

func (r *CharacterRepository) ListByCampaign(ctx context.Context, campaignID string) ([]character.Character, error) {
	rows, err := r.store.db.QueryContext(ctx, `
		SELECT id, campaign_id, name, role, level, experience, health, movement, armor, notes, created_at, updated_at
		FROM characters
		WHERE campaign_id = ?
		ORDER BY name COLLATE NOCASE ASC`, campaignID)
	if err != nil {
		return nil, fmt.Errorf("list characters: %w", err)
	}
	defer rows.Close()
	var characters []character.Character
	for rows.Next() {
		ch, err := scanCharacter(rows)
		if err != nil {
			return nil, err
		}
		if err := loadCharacterDetails(ctx, r.store.db, ch); err != nil {
			return nil, err
		}
		characters = append(characters, *ch)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate characters: %w", err)
	}
	return characters, nil
}

type characterScanner interface {
	Scan(dest ...any) error
}

func ensureCharacterDefaults(c *character.Character, now time.Time) error {
	if c.ID == "" {
		id, err := platform.NewID()
		if err != nil {
			return err
		}
		c.ID = id
	}
	c.CreatedAt = nowIfZero(c.CreatedAt, now)
	c.UpdatedAt = nowIfZero(c.UpdatedAt, now)
	if c.CustomFields == nil {
		c.CustomFields = map[string]string{}
	}
	for i := range c.Equipment {
		if c.Equipment[i].ID == "" {
			id, err := platform.NewID()
			if err != nil {
				return err
			}
			c.Equipment[i].ID = id
		}
		c.Equipment[i].CharacterID = c.ID
		c.Equipment[i].CreatedAt = nowIfZero(c.Equipment[i].CreatedAt, now)
		c.Equipment[i].UpdatedAt = nowIfZero(c.Equipment[i].UpdatedAt, now)
		if c.Equipment[i].Quantity == 0 {
			c.Equipment[i].Quantity = 1
		}
	}
	for i := range c.Traits {
		if c.Traits[i].ID == "" {
			id, err := platform.NewID()
			if err != nil {
				return err
			}
			c.Traits[i].ID = id
		}
		c.Traits[i].CharacterID = c.ID
		c.Traits[i].CreatedAt = nowIfZero(c.Traits[i].CreatedAt, now)
		c.Traits[i].UpdatedAt = nowIfZero(c.Traits[i].UpdatedAt, now)
	}
	for i := range c.Injuries {
		if c.Injuries[i].ID == "" {
			id, err := platform.NewID()
			if err != nil {
				return err
			}
			c.Injuries[i].ID = id
		}
		c.Injuries[i].CharacterID = c.ID
		c.Injuries[i].CreatedAt = nowIfZero(c.Injuries[i].CreatedAt, now)
		c.Injuries[i].UpdatedAt = nowIfZero(c.Injuries[i].UpdatedAt, now)
	}
	return nil
}

func insertCharacter(ctx context.Context, exec sqlExecer, c *character.Character) error {
	if _, err := exec.ExecContext(ctx, `
		INSERT INTO characters(id, campaign_id, name, role, level, experience, health, movement, armor, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.CampaignID, c.Name, c.Role, c.Level, c.Experience, c.Health, c.Movement, c.Armor, c.Notes, formatTime(c.CreatedAt), formatTime(c.UpdatedAt),
	); err != nil {
		return fmt.Errorf("insert character %s: %w", c.ID, err)
	}
	return nil
}

func selectCharacter(ctx context.Context, queryer sqlQueryer, id string) (*character.Character, error) {
	row := queryer.QueryRowContext(ctx, `
		SELECT id, campaign_id, name, role, level, experience, health, movement, armor, notes, created_at, updated_at
		FROM characters
		WHERE id = ?`, id)
	ch, err := scanCharacter(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, campaign.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func scanCharacter(scanner characterScanner) (*character.Character, error) {
	var c character.Character
	var createdAt, updatedAt string
	if err := scanner.Scan(&c.ID, &c.CampaignID, &c.Name, &c.Role, &c.Level, &c.Experience, &c.Health, &c.Movement, &c.Armor, &c.Notes, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scan character: %w", err)
	}
	c.CreatedAt = parseTime(createdAt)
	c.UpdatedAt = parseTime(updatedAt)
	c.CustomFields = map[string]string{}
	return &c, nil
}

func replaceCharacterDetails(ctx context.Context, tx *sql.Tx, c *character.Character) error {
	for _, table := range []string{"equipment", "traits", "injuries", "custom_fields"} {
		if _, err := tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE character_id = ?", c.ID); err != nil {
			return fmt.Errorf("clear %s for character %s: %w", table, c.ID, err)
		}
	}
	for _, item := range c.Equipment {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO equipment(id, character_id, name, quantity, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.ID, c.ID, item.Name, item.Quantity, item.Notes, formatTime(item.CreatedAt), formatTime(item.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert equipment %s: %w", item.ID, err)
		}
	}
	for _, trait := range c.Traits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO traits(id, character_id, name, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?)`,
			trait.ID, c.ID, trait.Name, trait.Notes, formatTime(trait.CreatedAt), formatTime(trait.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert trait %s: %w", trait.ID, err)
		}
	}
	for _, injury := range c.Injuries {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO injuries(id, character_id, name, recovered, notes, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			injury.ID, c.ID, injury.Name, boolInt(injury.Recovered), injury.Notes, formatTime(injury.CreatedAt), formatTime(injury.UpdatedAt),
		); err != nil {
			return fmt.Errorf("insert injury %s: %w", injury.ID, err)
		}
	}
	for key, value := range c.CustomFields {
		if _, err := tx.ExecContext(ctx, `INSERT INTO custom_fields(character_id, key, value) VALUES (?, ?, ?)`, c.ID, key, value); err != nil {
			return fmt.Errorf("insert custom field %q: %w", key, err)
		}
	}
	return nil
}

func loadCharacterDetails(ctx context.Context, queryer sqlQueryer, c *character.Character) error {
	equipment, err := loadEquipment(ctx, queryer, c.ID)
	if err != nil {
		return err
	}
	traits, err := loadTraits(ctx, queryer, c.ID)
	if err != nil {
		return err
	}
	injuries, err := loadInjuries(ctx, queryer, c.ID)
	if err != nil {
		return err
	}
	fields, err := loadCustomFields(ctx, queryer, c.ID)
	if err != nil {
		return err
	}
	c.Equipment = equipment
	c.Traits = traits
	c.Injuries = injuries
	c.CustomFields = fields
	return nil
}

func loadEquipment(ctx context.Context, queryer sqlQueryer, characterID string) ([]character.EquipmentItem, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, character_id, name, quantity, notes, created_at, updated_at
		FROM equipment WHERE character_id = ? ORDER BY name COLLATE NOCASE ASC`, characterID)
	if err != nil {
		return nil, fmt.Errorf("load equipment: %w", err)
	}
	defer rows.Close()
	var items []character.EquipmentItem
	for rows.Next() {
		var item character.EquipmentItem
		var createdAt, updatedAt string
		if err := rows.Scan(&item.ID, &item.CharacterID, &item.Name, &item.Quantity, &item.Notes, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan equipment: %w", err)
		}
		item.CreatedAt = parseTime(createdAt)
		item.UpdatedAt = parseTime(updatedAt)
		items = append(items, item)
	}
	return items, rows.Err()
}

func loadTraits(ctx context.Context, queryer sqlQueryer, characterID string) ([]character.Trait, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, character_id, name, notes, created_at, updated_at
		FROM traits WHERE character_id = ? ORDER BY name COLLATE NOCASE ASC`, characterID)
	if err != nil {
		return nil, fmt.Errorf("load traits: %w", err)
	}
	defer rows.Close()
	var traits []character.Trait
	for rows.Next() {
		var trait character.Trait
		var createdAt, updatedAt string
		if err := rows.Scan(&trait.ID, &trait.CharacterID, &trait.Name, &trait.Notes, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan trait: %w", err)
		}
		trait.CreatedAt = parseTime(createdAt)
		trait.UpdatedAt = parseTime(updatedAt)
		traits = append(traits, trait)
	}
	return traits, rows.Err()
}

func loadInjuries(ctx context.Context, queryer sqlQueryer, characterID string) ([]character.Injury, error) {
	rows, err := queryer.QueryContext(ctx, `
		SELECT id, character_id, name, recovered, notes, created_at, updated_at
		FROM injuries WHERE character_id = ? ORDER BY name COLLATE NOCASE ASC`, characterID)
	if err != nil {
		return nil, fmt.Errorf("load injuries: %w", err)
	}
	defer rows.Close()
	var injuries []character.Injury
	for rows.Next() {
		var injury character.Injury
		var recovered int
		var createdAt, updatedAt string
		if err := rows.Scan(&injury.ID, &injury.CharacterID, &injury.Name, &recovered, &injury.Notes, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan injury: %w", err)
		}
		injury.Recovered = intBool(recovered)
		injury.CreatedAt = parseTime(createdAt)
		injury.UpdatedAt = parseTime(updatedAt)
		injuries = append(injuries, injury)
	}
	return injuries, rows.Err()
}

func loadCustomFields(ctx context.Context, queryer sqlQueryer, characterID string) (map[string]string, error) {
	rows, err := queryer.QueryContext(ctx, `SELECT key, value FROM custom_fields WHERE character_id = ? ORDER BY key COLLATE NOCASE ASC`, characterID)
	if err != nil {
		return nil, fmt.Errorf("load custom fields: %w", err)
	}
	defer rows.Close()
	fields := map[string]string{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan custom field: %w", err)
		}
		fields[key] = value
	}
	return fields, rows.Err()
}
