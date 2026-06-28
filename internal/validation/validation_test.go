package validation

import (
	"errors"
	"testing"

	"warband-vault/internal/campaign"
	"warband-vault/internal/character"
)

func TestValidateCampaignNameRequired(t *testing.T) {
	err := ValidateCampaign(&campaign.Campaign{Name: "   "})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid campaign, got %v", err)
	}
}

func TestValidateCharacterRejectsNegativeFields(t *testing.T) {
	err := ValidateCharacter(&character.Character{Name: "Mara", Level: -1})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid character, got %v", err)
	}
}

func TestValidateCharacterRejectsBlankCustomFieldKey(t *testing.T) {
	err := ValidateCharacter(&character.Character{
		Name:         "Mara",
		CustomFields: map[string]string{" ": "value"},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("expected invalid custom field, got %v", err)
	}
}
