package campaign

import (
	"time"

	"warband-vault/internal/character"
)

type Campaign struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	SystemName  string                `json:"system_name"`
	Description string                `json:"description"`
	Treasury    int                   `json:"treasury"`
	Archived    bool                  `json:"archived"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Characters  []character.Character `json:"characters"`
}
