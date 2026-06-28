package character

import "time"

type EquipmentItem struct {
	ID          string    `json:"id"`
	CharacterID string    `json:"character_id,omitempty"`
	Name        string    `json:"name"`
	Quantity    int       `json:"quantity"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Trait struct {
	ID          string    `json:"id"`
	CharacterID string    `json:"character_id,omitempty"`
	Name        string    `json:"name"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Injury struct {
	ID          string    `json:"id"`
	CharacterID string    `json:"character_id,omitempty"`
	Name        string    `json:"name"`
	Recovered   bool      `json:"recovered"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Character struct {
	ID           string            `json:"id"`
	CampaignID   string            `json:"campaign_id,omitempty"`
	Name         string            `json:"name"`
	Role         string            `json:"role"`
	Level        int               `json:"level"`
	Experience   int               `json:"experience"`
	Health       int               `json:"health"`
	Movement     int               `json:"movement"`
	Armor        int               `json:"armor"`
	Notes        string            `json:"notes"`
	Equipment    []EquipmentItem   `json:"equipment"`
	Traits       []Trait           `json:"traits"`
	Injuries     []Injury          `json:"injuries"`
	CustomFields map[string]string `json:"custom_fields"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}
