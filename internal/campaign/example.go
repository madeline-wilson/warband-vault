package campaign

import "warband-vault/internal/character"

func ExampleBlackwaterExpedition() Campaign {
	return Campaign{
		Name:        "The Blackwater Expedition",
		SystemName:  "Generic Skirmish",
		Description: "A small band follows old river maps into a drowned borderland in search of lost tollhouse gold.",
		Treasury:    35,
		Characters: []character.Character{
			{
				Name:       "Mara Voss",
				Role:       "Scout",
				Level:      2,
				Experience: 8,
				Health:     10,
				Movement:   6,
				Armor:      1,
				Equipment: []character.EquipmentItem{
					{Name: "Short bow", Quantity: 1},
					{Name: "Oil lantern", Quantity: 1},
				},
				Traits:       []character.Trait{{Name: "Sure-footed", Notes: "Ignores the first movement penalty in marsh terrain."}},
				Injuries:     []character.Injury{{Name: "Old scar", Notes: "No current penalty."}},
				Notes:        "Keeps the expedition moving when the paths disappear under water.",
				CustomFields: map[string]string{"oath": "Find the sunken ford"},
			},
			{
				Name:       "Brother Hal",
				Role:       "Field medic",
				Level:      1,
				Experience: 3,
				Health:     9,
				Movement:   5,
				Armor:      2,
				Equipment: []character.EquipmentItem{
					{Name: "Bandage roll", Quantity: 3},
					{Name: "Iron mace", Quantity: 1},
				},
				Traits:       []character.Trait{{Name: "Steady hands"}},
				Notes:        "Former shrine keeper with a practical view of omens.",
				CustomFields: map[string]string{"favor": "River chapel"},
			},
		},
	}
}
